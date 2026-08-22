// Package workers provides a bounded in-process worker pool with retry,
// tracing, and mandatory observability through webFramework.AddLog.
//
// Workers run outside HTTP request contexts, so each job receives a
// job-owned webFramework.WebFramework backed by a concurrency-safe
// BackgroundParser. This ensures that webFramework.AddLog calls
// (including those from external API calls via handlers.CallAPI) flow
// into the Splunk transaction pipeline.
package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

// JobHandler is the function executed by a worker job.
// The JobContext provides a context and a job-owned webFramework.WebFramework
// for mandatory AddLog calls.
type JobHandler func(*JobContext) error

// JobContext provides the execution context for a worker job.
type JobContext struct {
	// Context carries cancellation and tracing.
	Context context.Context

	// WebFramework is a job-owned root webFramework.WebFramework with a
	// BackgroundParser that supports webFramework.AddLog calls. This
	// ensures that external API calls and critical business events
	// within jobs are logged to the Splunk transaction pipeline.
	WebFramework webFramework.WebFramework

	// JobName is the name of the job.
	JobName string

	// Attempt is the current attempt number (1-based).
	Attempt int

	// Attributes are tracing attributes for the job.
	Attributes map[string]string

	// transactionSink collects AddLog entries for this job attempt.
	// It is flushed after each attempt to emit the mandatory
	// worker-<name>-req / worker-<name>-req-failed log entries.
	transactionSink *TransactionSink
}

// TransactionSink collects AddLog entries emitted during a job attempt
// and flushes them as a single transaction log entry after the attempt
// completes. This ensures worker observability even when the job handler
// does not explicitly collect logs.
type TransactionSink struct {
	mu      sync.Mutex
	entries []slog.Attr
}

// NewTransactionSink creates a new empty TransactionSink.
func NewTransactionSink() *TransactionSink {
	return &TransactionSink{}
}

// Add appends a log attribute to the sink.
func (s *TransactionSink) Add(attr slog.Attr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, attr)
}

// Entries returns a copy of the collected log attributes.
func (s *TransactionSink) Entries() []slog.Attr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]slog.Attr(nil), s.entries...)
}

// Reset clears the sink.
func (s *TransactionSink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
}

// Job defines a unit of asynchronous work.
type Job struct {
	// Name identifies the job for logging and tracing.
	Name string

	// Handler is the function to execute.
	Handler JobHandler

	// Options configures retry, backoff, and tracing.
	Options JobOptions
}

// JobOptions configures job execution behavior.
type JobOptions struct {
	// MaxAttempts is the maximum number of execution attempts (including the first).
	// Default: 1 (no retry).
	MaxAttempts int

	// InitialBackoff is the delay before the first retry.
	// Default: 100ms.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between retries.
	// Default: 5s.
	MaxBackoff time.Duration

	// Jitter adds randomness to backoff to prevent thundering herd.
	// Default: true.
	Jitter bool

	// Attributes are tracing attributes for the job span.
	Attributes map[string]string

	// OnFailure is called when all attempts are exhausted.
	// It receives the final error and the total number of attempts.
	OnFailure func(err error, attempts int)

	// PropagateCancel, when true, makes the job context derive from
	// the submit context so cancellation propagates. When false
	// (default), the job context uses context.WithoutCancel so
	// the job outlives the request.
	PropagateCancel bool
}

// Stats holds worker pool statistics.
type Stats struct {
	Submitted  int64
	Succeeded  int64
	Failed     int64
	InFlight   int64
	QueueDepth int
	Workers    int
}

// Worker is the interface for submitting and managing background jobs.
type Worker interface {
	// Submit enqueues a job for asynchronous execution.
	// Returns an error if the queue is full or the worker is shutting down.
	Submit(ctx context.Context, job Job) error

	// Shutdown stops accepting new jobs, drains the queue, and waits
	// for in-flight jobs to complete or the context to expire.
	Shutdown(ctx context.Context) error

	// Stats returns current worker pool statistics.
	Stats() Stats
}

// Config configures an InProcessWorker.
type Config struct {
	// WorkerCount is the number of goroutines processing jobs.
	// Default: runtime.NumCPU().
	WorkerCount int

	// QueueSize is the buffered channel capacity.
	// Default: 100.
	QueueSize int

	// BlockOnFull determines whether Submit blocks when the queue is full
	// (true) or returns ErrQueueFull immediately (false).
	// Default: false.
	BlockOnFull bool

	// Clock is the clock source for deterministic testing.
	// If nil, time.Now is used.
	Clock func() time.Time

	// JitterSource is the jitter source for deterministic testing.
	// If nil, a package-level locked random source is used.
	JitterSource func(max int64) int64
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		WorkerCount: 0, // will be set to NumCPU
		QueueSize:   100,
		BlockOnFull: false,
	}
}

// ErrQueueFull is returned when the job queue is full and BlockOnFull is false.
var ErrQueueFull = errQueueFull{}

type errQueueFull struct{}

func (errQueueFull) Error() string { return "workers: queue is full" }
func (errQueueFull) Is(target error) bool {
	_, ok := target.(errQueueFull)
	return ok
}

// ErrShutdown is returned when Submit is called after Shutdown has begun.
var ErrShutdown = errShutdown{}

type errShutdown struct{}

func (errShutdown) Error() string { return "workers: pool is shutting down" }
func (errShutdown) Is(target error) bool {
	_, ok := target.(errShutdown)
	return ok
}

// ErrInvalidJob is returned when a job has an empty name or nil handler.
var ErrInvalidJob = errors.New("workers: invalid job (empty name or nil handler)")

// CallAPILogEntry mirrors the v1 handlers.CallAPILogEntry constant so
// worker jobs can collect API-call log arrays in the finalizer without
// importing the v1 handlers package directly.
const CallAPILogEntry string = "ApiCall"

// jobEnvelope carries a Job and its submission context through the queue
// so that context values and tracing survive into the worker goroutine.
type jobEnvelope struct {
	job       Job
	submitCtx context.Context
}

// InProcessWorker is a bounded goroutine pool implementation of Worker.
type InProcessWorker struct {
	config   Config
	queue    chan jobEnvelope
	wg       sync.WaitGroup
	shutdown atomic.Bool
	shutOnce sync.Once
	stats    Stats
	mu       sync.Mutex

	// shutDone is cached on first Shutdown call to avoid creating a
	// waiter goroutine on every Shutdown invocation.
	shutDoneOnce sync.Once
	shutDone     chan struct{}

	// clock and jitter sources for deterministic testing.
	clock        func() time.Time
	jitterSource func(max int64) int64
}

// NewInProcessWorker creates a new InProcessWorker with the given configuration.
func NewInProcessWorker(config Config) *InProcessWorker {
	if config.WorkerCount <= 0 {
		config.WorkerCount = getNumCPU()
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	w := &InProcessWorker{
		config:       config,
		queue:        make(chan jobEnvelope, config.QueueSize),
		shutDone:     make(chan struct{}),
		clock:        config.Clock,
		jitterSource: config.JitterSource,
	}
	if w.clock == nil {
		w.clock = time.Now
	}
	if w.jitterSource == nil {
		w.jitterSource = defaultJitter
	}
	w.start()
	return w
}

func (w *InProcessWorker) start() {
	for i := 0; i < w.config.WorkerCount; i++ {
		w.wg.Add(1)
		go w.workerLoop()
	}
}

func (w *InProcessWorker) workerLoop() {
	defer w.wg.Done()
	for env := range w.queue {
		atomic.AddInt64(&w.stats.InFlight, 1)
		w.executeJob(env)
		atomic.AddInt64(&w.stats.InFlight, -1)
	}
}

func (w *InProcessWorker) executeJob(env jobEnvelope) {
	job := env.job
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

	// Derive the job context from the submission context.
	// By default, use context.WithoutCancel so values/tracing survive
	// without cancellation. When PropagateCancel is true, use the
	// submit context directly so cancellation propagates.
	var jobCtx context.Context
	if opts.PropagateCancel {
		jobCtx = env.submitCtx
	} else {
		jobCtx = context.WithoutCancel(env.submitCtx)
	}
	if jobCtx == nil {
		jobCtx = context.Background()
	}

	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		// Build the job context with a BackgroundParser and TransactionSink.
		sink := NewTransactionSink()
		bgParser := newBackgroundParser(sink, jobCtx)

		wf := webFramework.WebFramework{
			Parser: bgParser,
			Ctx:    jobCtx,
		}

		jctx := &JobContext{
			Context:         jobCtx,
			WebFramework:    wf,
			JobName:         job.Name,
			Attempt:         attempt,
			Attributes:      opts.Attributes,
			transactionSink: sink,
		}

		start := w.clock()
		err := w.runWithObservability(jctx, job.Handler)
		elapsed := w.clock().Sub(start)

		// Collect logs from the parser into the transaction sink.
		webFramework.CollectLogArrays(wf, webFramework.HandlerLogTag)
		webFramework.CollectLogTags(wf, webFramework.HandlerLogTag)
		webFramework.CollectLogArrays(wf, CallAPILogEntry)

		// Flush the transaction sink with mandatory AddLog entries.
		w.flushTransaction(jctx, err, elapsed)

		if err == nil {
			atomic.AddInt64(&w.stats.Succeeded, 1)
			return
		}

		lastErr = err

		if attempt < opts.MaxAttempts {
			backoff := calculateBackoff(opts.InitialBackoff, opts.MaxBackoff, attempt, opts.Jitter, w.jitterSource)
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-jobCtx.Done():
				timer.Stop()
				// Cancellation is a terminal failure.
				atomic.AddInt64(&w.stats.Failed, 1)
				if opts.OnFailure != nil {
					w.runOnFailure(opts.OnFailure, jobCtx.Err(), attempt)
				}
				return
			}
		}
	}

	atomic.AddInt64(&w.stats.Failed, 1)
	if opts.OnFailure != nil {
		w.runOnFailure(opts.OnFailure, lastErr, opts.MaxAttempts)
	}
}

// flushTransaction emits the mandatory worker-<name>-req (success) or
// worker-<name>-req-failed (failure) log entry with attempt, elapsed time,
// terminal state, and collected transaction attributes.
//
// The outcome entry is emitted through webFramework.AddLog on the job-owned
// WebFramework so it flows into the Splunk transaction pipeline alongside
// the handler's own AddLog entries. The entry is then collected into the
// transaction sink via CollectLogArrays and also emitted via slog as a
// supplementary log for environments without a Splunk-connected handler.
func (w *InProcessWorker) flushTransaction(ctx *JobContext, err error, elapsed time.Duration) {
	if ctx.transactionSink == nil {
		return
	}

	key := "worker-" + ctx.JobName + "-req"
	if err != nil {
		key = "worker-" + ctx.JobName + "-req-failed"
	}

	// Build the outcome attributes.
	outcomeAttrs := make([]slog.Attr, 0, 4)
	outcomeAttrs = append(outcomeAttrs, slog.Int("attempt", ctx.Attempt))
	outcomeAttrs = append(outcomeAttrs, slog.String("elapsed", elapsed.String()))
	if err != nil {
		outcomeAttrs = append(outcomeAttrs, slog.String("error", err.Error()))
		outcomeAttrs = append(outcomeAttrs, slog.String("state", "failed"))
	} else {
		outcomeAttrs = append(outcomeAttrs, slog.String("state", "succeeded"))
	}

	// Emit the outcome entry through the real webFramework.AddLog pipeline
	// so it flows into the Splunk transaction pipeline. This satisfies the
	// mandatory AddLog requirement for worker transaction boundaries.
	if ctx.WebFramework.Parser != nil {
		for _, attr := range outcomeAttrs {
			webFramework.AddLog(ctx.WebFramework, key, attr)
		}
		// Collect the outcome entry into the transaction sink.
		webFramework.CollectLogArrays(ctx.WebFramework, key)
	}

	// Read all sink entries (handler AddLog entries + outcome entry).
	entries := ctx.transactionSink.Entries()

	// Build the full attribute list for the supplementary slog emission.
	attrs := make([]slog.Attr, 0, len(entries)+len(outcomeAttrs))
	attrs = append(attrs, outcomeAttrs...)
	attrs = append(attrs, entries...)

	// Emit via slog as a supplementary log. The real AddLog entry is
	// already in the pipeline via the AddLog calls above; this slog
	// emission ensures observability in environments without a
	// Splunk-connected slog handler.
	slog.LogAttrs(context.Background(), slog.LevelInfo, key, attrs...)
}

func (w *InProcessWorker) runWithObservability(ctx *JobContext, handler JobHandler) (err error) {
	// Recover panics as job failures; never kill worker goroutines.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("workers: job %q panicked: %v", ctx.JobName, r)
		}
	}()
	return handler(ctx)
}

func (w *InProcessWorker) runOnFailure(fn func(error, int), err error, attempts int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("workers: OnFailure callback panicked",
				slog.Any("panic", r))
		}
	}()
	fn(err, attempts)
}

// Submit enqueues a job for asynchronous execution. Returns an error if
// the queue is full, the worker is shutting down, or the job is invalid.
// Only accepted submissions increment the Submitted counter.
//
// The shutdown check and queue send are synchronized under mu to prevent
// a send on a closed channel when Shutdown runs concurrently.
func (w *InProcessWorker) Submit(ctx context.Context, job Job) error {
	if job.Name == "" || job.Handler == nil {
		return ErrInvalidJob
	}
	if ctx == nil {
		ctx = context.Background()
	}

	env := jobEnvelope{job: job, submitCtx: ctx}

	// Synchronized admission: check shutdown under the same lock that
	// guards the queue send. This prevents the check-then-close race
	// where a goroutine sees shutdown=false, then Shutdown closes the
	// channel, and the goroutine sends on the closed channel.
	w.mu.Lock()
	if w.shutdown.Load() {
		w.mu.Unlock()
		return ErrShutdown
	}

	// Count only accepted submissions.
	atomic.AddInt64(&w.stats.Submitted, 1)

	if w.config.BlockOnFull {
		// BlockOnFull: send while holding the lock to prevent
		// a close during the send. Use a select to respect context
		// cancellation.
		select {
		case w.queue <- env:
			w.mu.Unlock()
			return nil
		case <-ctx.Done():
			w.mu.Unlock()
			// Decrement since we counted it above but won't enqueue.
			atomic.AddInt64(&w.stats.Submitted, -1)
			return ctx.Err()
		}
	}

	// Non-blocking: try to send, return ErrQueueFull if full.
	select {
	case w.queue <- env:
		w.mu.Unlock()
		return nil
	default:
		w.mu.Unlock()
		// Decrement since we counted it above but won't enqueue.
		atomic.AddInt64(&w.stats.Submitted, -1)
		// Check if shutdown started while we were trying to send.
		if w.shutdown.Load() {
			return ErrShutdown
		}
		return ErrQueueFull
	}
}

// Shutdown stops accepting new jobs, drains the queue, and waits for
// in-flight jobs to complete or the context to expire. Shutdown is
// idempotent; calling it multiple times is safe and returns the same
// result.
func (w *InProcessWorker) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	w.shutOnce.Do(func() {
		// Hold the mutex while setting shutdown and closing the queue
		// to prevent Submit from sending on a closed channel.
		w.mu.Lock()
		w.shutdown.Store(true)
		close(w.queue)
		w.mu.Unlock()
	})

	// Cache the completion channel so repeated Shutdown calls don't
	// create new waiter goroutines.
	w.shutDoneOnce.Do(func() {
		go func() {
			w.wg.Wait()
			close(w.shutDone)
		}()
	})

	select {
	case <-w.shutDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns current worker pool statistics.
func (w *InProcessWorker) Stats() Stats {
	return Stats{
		Submitted:  atomic.LoadInt64(&w.stats.Submitted),
		Succeeded:  atomic.LoadInt64(&w.stats.Succeeded),
		Failed:     atomic.LoadInt64(&w.stats.Failed),
		InFlight:   atomic.LoadInt64(&w.stats.InFlight),
		QueueDepth: len(w.queue),
		Workers:    w.config.WorkerCount,
	}
}
