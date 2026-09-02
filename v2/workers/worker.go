// Package workers provides a bounded in-process worker pool with retry,
// tracing, and mandatory observability through telemetry.Sink.
//
// Workers run outside HTTP request contexts, so each job receives a
// job-scoped *slog.Logger and a telemetry.Sink for recording lifecycle
// events. The TransactionSink collects slog attributes emitted during a
// job attempt via a slog.Handler, teeing them to the configured logger
// while accumulating them for inclusion in the final telemetry event.
package workers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
)

// JobHandler is the function executed by a worker job.
// The JobContext provides a context, a scoped *slog.Logger, and a
// telemetry.Sink for recording lifecycle events.
type JobHandler func(*JobContext) error

// JobContext provides the execution context for a worker job.
type JobContext struct {
	// Context carries cancellation and tracing.
	Context context.Context

	// Logger is a job-scoped *slog.Logger backed by a TransactionSink
	// handler. Logs emitted through this logger are collected for the
	// transaction pipeline and tee'd to the configured logger.
	Logger *slog.Logger

	// Sink is the telemetry sink for recording lifecycle events.
	Sink telemetry.Sink

	// JobName is the name of the job.
	JobName string

	// Attempt is the current attempt number (1-based).
	Attempt int

	// Attributes are tracing attributes for the job.
	Attributes map[string]string

	// transactionSink collects slog attributes emitted during a job
	// attempt via its slog.Handler interface. It is read after each
	// attempt to include collected attributes in the telemetry event.
	transactionSink *TransactionSink
}

// transactionSinkState holds the shared collection state for a
// TransactionSink and all handlers derived from it via WithAttrs.
type transactionSinkState struct {
	mu      sync.Mutex
	entries []slog.Attr
}

// TransactionSink is a concurrency-safe slog.Handler that collects
// attributes from log records emitted during a job attempt and tees
// each record to an underlying logger handler. It replaces the v1
// BackgroundParser bridge: instead of storing AddLog entries in parser
// locals, it collects them through the standard slog.Handler interface.
//
// All handlers derived from a TransactionSink via WithAttrs share the
// same collection state, so attributes logged through child loggers
// (e.g. logger.With("key", "val")) are visible in Entries().
type TransactionSink struct {
	state *transactionSinkState
	pre   []slog.Attr
	tee   slog.Handler
}

// NewTransactionSink creates a new TransactionSink that tees records to
// the given logger. If logger is nil, slog.Default() is used.
func NewTransactionSink(logger *slog.Logger) *TransactionSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &TransactionSink{
		state: &transactionSinkState{},
		tee:   logger.Handler(),
	}
}

// Handle implements slog.Handler. It collects the record's attributes
// (plus any pre-attributes from WithAttrs) into the shared state and
// tees the record to the underlying logger handler if it is enabled
// for the record's level.
func (s *TransactionSink) Handle(ctx context.Context, r slog.Record) error {
	s.state.mu.Lock()
	for _, a := range s.pre {
		s.state.entries = append(s.state.entries, a)
	}
	r.Attrs(func(attr slog.Attr) bool {
		s.state.entries = append(s.state.entries, attr)
		return true
	})
	s.state.mu.Unlock()
	// Only tee to the underlying handler if it is enabled for this level.
	if s.tee.Enabled(ctx, r.Level) {
		return s.tee.Handle(ctx, r.Clone())
	}
	return nil
}

// WithAttrs returns a new TransactionSink that shares the same
// collection state but prepends the given attributes to each record.
func (s *TransactionSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TransactionSink{
		state: s.state,
		pre:   append(append([]slog.Attr{}, s.pre...), attrs...),
		tee:   s.tee,
	}
}

// WithGroup returns the receiver unchanged. Group nesting is not
// supported for attribute collection.
func (s *TransactionSink) WithGroup(name string) slog.Handler {
	return s
}

// Enabled implements slog.Handler. It always returns true so that
// attributes are collected regardless of the underlying logger's level
// filtering. The tee to the underlying handler is gated separately in
// Handle.
func (s *TransactionSink) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Entries returns a copy of the collected log attributes from all
// handlers sharing this sink's state.
func (s *TransactionSink) Entries() []slog.Attr {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return slices.Clone(s.state.entries)
}

// Reset clears the collected log attributes.
func (s *TransactionSink) Reset() {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.entries = nil
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

	// Logger is the base logger for worker jobs. Each job receives a
	// scoped logger backed by a TransactionSink that tees to this logger.
	// If nil, slog.Default() is used.
	Logger *slog.Logger

	// Sink is the telemetry sink for recording job lifecycle events.
	// If nil, telemetry.NopSink{} is used (no observability).
	Sink telemetry.Sink
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

	// logger is the base logger for job-scoped loggers.
	logger *slog.Logger

	// sink is the telemetry sink for recording job lifecycle events.
	sink telemetry.Sink
}

// NewInProcessWorker creates a new InProcessWorker with the given configuration.
func NewInProcessWorker(config Config) *InProcessWorker {
	if config.WorkerCount <= 0 {
		config.WorkerCount = getNumCPU()
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Sink == nil {
		config.Sink = telemetry.NopSink{}
	}
	w := &InProcessWorker{
		config:       config,
		queue:        make(chan jobEnvelope, config.QueueSize),
		shutDone:     make(chan struct{}),
		clock:        config.Clock,
		jitterSource: config.JitterSource,
		logger:       config.Logger,
		sink:         config.Sink,
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
		// Build the job-scoped logger and transaction sink.
		txSink := NewTransactionSink(w.logger)
		jobLogger := slog.New(txSink)

		jctx := &JobContext{
			Context:         jobCtx,
			Logger:          jobLogger,
			Sink:            w.sink,
			JobName:         job.Name,
			Attempt:         attempt,
			Attributes:      opts.Attributes,
			transactionSink: txSink,
		}

		start := w.clock()
		err := w.runWithObservability(jctx, job.Handler)
		elapsed := w.clock().Sub(start)

		// Record the telemetry event for this attempt.
		w.recordEvent(jctx, err, elapsed)

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

// recordEvent emits a telemetry.Event for the completed job attempt.
// Success events use EventSuccess with status 200 and operation
// "worker-<name>-req". Failure events use EventFailure with the error
// and operation "worker-<name>-req-failed". Collected transaction sink
// attributes are included in the event.
func (w *InProcessWorker) recordEvent(ctx *JobContext, err error, elapsed time.Duration) {
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

	w.sink.Record(telemetry.Event{
		Type:      eventType,
		Operation: operation,
		Status:    status,
		Duration:  elapsed,
		Err:       err,
		Attrs:     attrs,
	})
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
