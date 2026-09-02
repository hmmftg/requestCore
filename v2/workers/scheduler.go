package workers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
)

// ScheduledJob defines a periodic background task run by a Scheduler.
// Unlike InProcessWorker jobs (which are submitted once and executed
// asynchronously), a ScheduledJob runs its Handler at a fixed Interval
// until the Scheduler is shut down.
//
// Each tick of the scheduler creates a fresh JobContext with a scoped
// *slog.Logger and TransactionSink, providing the same telemetry.Sink
// observability as InProcessWorker jobs.
type ScheduledJob struct {
	// Name identifies the job for logging and tracing.
	Name string

	// Handler is the function executed on each tick.
	Handler JobHandler

	// Interval is the time between successive ticks.
	// Must be > 0.
	Interval time.Duration

	// Options configures retry, backoff, and tracing for each tick.
	// Retry applies within a single tick: if the handler returns an
	// error, it is retried up to MaxAttempts with backoff before
	// the next tick interval begins.
	Options JobOptions
}

// SchedulerStats holds per-job statistics for a Scheduler.
type SchedulerStats struct {
	Ticks     int64
	Succeeded int64
	Failed    int64
	LastRun   time.Time
	LastErr   string
	InFlight  bool
	NextRun   time.Time
}

// SchedulerConfig configures a Scheduler.
type SchedulerConfig struct {
	// Logger is the base logger for scheduled job ticks. Each tick
	// receives a scoped logger backed by a TransactionSink that tees
	// to this logger. If nil, slog.Default() is used.
	Logger *slog.Logger

	// Sink is the telemetry sink for recording tick lifecycle events.
	// If nil, telemetry.NopSink{} is used (no observability).
	Sink telemetry.Sink

	// Clock is the clock source for deterministic testing.
	// If nil, time.Now is used.
	Clock func() time.Time
}

// Scheduler runs periodic background tasks with telemetry.Sink
// observability. Each tick of each scheduled job receives a job-scoped
// *slog.Logger backed by a concurrency-safe TransactionSink, ensuring
// that log attributes flow into the telemetry pipeline.
//
// The Scheduler is designed for long-running poller loops (e.g.
// periodic data sync, health checks, cache refresh) that run at fixed
// intervals. For discrete, one-shot job submission, use InProcessWorker.
//
// The Scheduler is safe for concurrent use. Schedule can be called
// before or after Start, but jobs only begin ticking after Start is
// called. Shutdown stops all tickers and waits for in-flight ticks to
// complete or the context to expire.
type Scheduler struct {
	mu       sync.Mutex
	jobs     map[string]*scheduledJobEntry
	started  bool
	shutdown atomic.Bool
	shutOnce sync.Once
	wg       sync.WaitGroup

	// shutDone is closed when all job goroutines have exited.
	shutDone     chan struct{}
	shutDoneOnce sync.Once

	// clock is the clock source for deterministic testing.
	clock func() time.Time

	// logger is the base logger for tick-scoped loggers.
	logger *slog.Logger

	// sink is the telemetry sink for recording tick lifecycle events.
	sink telemetry.Sink
}

// scheduledJobEntry holds the runtime state for a registered job.
type scheduledJobEntry struct {
	job     ScheduledJob
	ticker  *time.Ticker
	stopCh  chan struct{}
	stats   SchedulerStats
	statsMu sync.Mutex
	clock   func() time.Time
}

// NewScheduler creates a new Scheduler with the given configuration.
// Jobs can be registered via Schedule before or after Start is called.
func NewScheduler(config SchedulerConfig) *Scheduler {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Sink == nil {
		config.Sink = telemetry.NopSink{}
	}
	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Scheduler{
		jobs:     make(map[string]*scheduledJobEntry),
		shutDone: make(chan struct{}),
		clock:    clock,
		logger:   config.Logger,
		sink:     config.Sink,
	}
}

// Schedule registers a periodic job. If the Scheduler has already
// started, the job begins ticking immediately. If the Scheduler has
// not started, the job begins ticking when Start is called.
//
// Returns an error if the job name is empty, the handler is nil, the
// interval is <= 0, or a job with the same name is already registered.
func (s *Scheduler) Schedule(job ScheduledJob) error {
	if job.Name == "" {
		return fmt.Errorf("workers: scheduled job name cannot be empty")
	}
	if job.Handler == nil {
		return fmt.Errorf("workers: scheduled job handler cannot be nil")
	}
	if job.Interval <= 0 {
		return fmt.Errorf("workers: scheduled job interval must be > 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shutdown.Load() {
		return ErrShutdown
	}

	if _, exists := s.jobs[job.Name]; exists {
		return fmt.Errorf("workers: scheduled job %q already registered", job.Name)
	}

	entry := &scheduledJobEntry{
		job:    job,
		stopCh: make(chan struct{}),
		clock:  s.clock,
	}

	s.jobs[job.Name] = entry

	if s.started {
		s.startJob(entry)
	}

	return nil
}

// Start begins ticking all registered jobs. Jobs registered after
// Start are started immediately by Schedule. Start is idempotent.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	for _, entry := range s.jobs {
		s.startJob(entry)
	}
}

// startJob launches the goroutine that ticks a single scheduled job.
// Must be called with s.mu held.
func (s *Scheduler) startJob(entry *scheduledJobEntry) {
	entry.ticker = time.NewTicker(entry.job.Interval)
	s.wg.Add(1)
	go s.runJob(entry)
}

// runJob is the per-job goroutine that ticks at the configured interval.
func (s *Scheduler) runJob(entry *scheduledJobEntry) {
	defer s.wg.Done()
	defer entry.ticker.Stop()

	for {
		select {
		case <-entry.ticker.C:
			s.executeTick(entry)
		case <-entry.stopCh:
			return
		}
	}
}

// executeTick runs a single tick of the scheduled job, with retry,
// observability, and panic recovery.
func (s *Scheduler) executeTick(entry *scheduledJobEntry) {
	job := entry.job
	opts := job.Options
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 1
	}
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 100 * time.Millisecond
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 5 * time.Second
	}
	if opts.Attributes == nil {
		opts.Attributes = make(map[string]string)
	}

	// Mark in-flight.
	entry.statsMu.Lock()
	entry.stats.InFlight = true
	entry.statsMu.Unlock()
	defer func() {
		entry.statsMu.Lock()
		entry.stats.InFlight = false
		entry.statsMu.Unlock()
	}()

	jobCtx := context.Background()

	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		// Build the tick-scoped logger and transaction sink.
		txSink := NewTransactionSink(s.logger)
		jobLogger := slog.New(txSink)

		jctx := &JobContext{
			Context:         jobCtx,
			Logger:          jobLogger,
			Sink:            s.sink,
			JobName:         job.Name,
			Attempt:         attempt,
			Attributes:      opts.Attributes,
			transactionSink: txSink,
		}

		start := s.clock()
		err := s.runWithObservability(jctx, job.Handler)
		elapsed := s.clock().Sub(start)

		// Record the telemetry event for this tick attempt.
		s.recordEvent(jctx, err, elapsed)

		if err == nil {
			atomic.AddInt64(&entry.stats.Ticks, 1)
			atomic.AddInt64(&entry.stats.Succeeded, 1)
			s.updateStats(entry, nil)
			return
		}

		lastErr = err

		if attempt < opts.MaxAttempts {
			backoff := calculateBackoff(opts.InitialBackoff, opts.MaxBackoff, attempt, opts.Jitter, defaultJitter)
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-entry.stopCh:
				timer.Stop()
				return
			}
		}
	}

	atomic.AddInt64(&entry.stats.Ticks, 1)
	atomic.AddInt64(&entry.stats.Failed, 1)
	s.updateStats(entry, lastErr)

	if opts.OnFailure != nil {
		s.runOnFailure(opts.OnFailure, lastErr, opts.MaxAttempts)
	}
}

// recordEvent emits a telemetry.Event for the completed tick attempt.
// Success events use EventSuccess with status 200 and operation
// "worker-<name>-req". Failure events use EventFailure with the error
// and operation "worker-<name>-req-failed". Collected transaction sink
// attributes are included in the event.
func (s *Scheduler) recordEvent(ctx *JobContext, err error, elapsed time.Duration) {
	operation := "worker-" + ctx.JobName + "-req"
	eventType := telemetry.EventSuccess
	status := 200
	if err != nil {
		operation = "worker-" + ctx.JobName + "-req-failed"
		eventType = telemetry.EventFailure
		status = 500
	}

	attrs := make([]slog.Attr, 0, 4)
	attrs = append(attrs, slog.Int("attempt", ctx.Attempt))
	attrs = append(attrs, slog.String("elapsed", elapsed.String()))
	if ctx.transactionSink != nil {
		attrs = append(attrs, ctx.transactionSink.Entries()...)
	}

	s.sink.Record(telemetry.Event{
		Type:      eventType,
		Operation: operation,
		Status:    status,
		Duration:  elapsed,
		Err:       err,
		Attrs:     attrs,
	})
}

// updateStats updates the per-job stats after a tick.
func (s *Scheduler) updateStats(entry *scheduledJobEntry, err error) {
	now := s.clock()
	entry.statsMu.Lock()
	entry.stats.LastRun = now
	if entry.job.Interval > 0 {
		entry.stats.NextRun = now.Add(entry.job.Interval)
	}
	if err != nil {
		entry.stats.LastErr = err.Error()
	} else {
		entry.stats.LastErr = ""
	}
	entry.statsMu.Unlock()
}

// runWithObservability runs the handler with panic recovery.
func (s *Scheduler) runWithObservability(ctx *JobContext, handler JobHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("workers: scheduled job %q panicked: %v", ctx.JobName, r)
		}
	}()
	return handler(ctx)
}

// runOnFailure runs the OnFailure callback with panic recovery.
func (s *Scheduler) runOnFailure(fn func(error, int), err error, attempts int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("workers: scheduled job OnFailure callback panicked",
				slog.Any("panic", r))
		}
	}()
	fn(err, attempts)
}

// Shutdown stops all scheduled jobs, waits for in-flight ticks to
// complete or the context to expire. Shutdown is idempotent.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.shutOnce.Do(func() {
		s.mu.Lock()
		s.shutdown.Store(true)
		for _, entry := range s.jobs {
			close(entry.stopCh)
		}
		s.mu.Unlock()
	})

	s.shutDoneOnce.Do(func() {
		go func() {
			s.wg.Wait()
			close(s.shutDone)
		}()
	})

	select {
	case <-s.shutDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns a map of job name to SchedulerStats for all registered
// jobs. The stats are a point-in-time snapshot and may be slightly
// stale for in-flight ticks.
func (s *Scheduler) Stats() map[string]SchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]SchedulerStats, len(s.jobs))
	for name, entry := range s.jobs {
		entry.statsMu.Lock()
		stats := SchedulerStats{
			Ticks:     atomic.LoadInt64(&entry.stats.Ticks),
			Succeeded: atomic.LoadInt64(&entry.stats.Succeeded),
			Failed:    atomic.LoadInt64(&entry.stats.Failed),
			InFlight:  entry.stats.InFlight,
			LastRun:   entry.stats.LastRun,
			LastErr:   entry.stats.LastErr,
			NextRun:   entry.stats.NextRun,
		}
		entry.statsMu.Unlock()
		result[name] = stats
	}
	return result
}
