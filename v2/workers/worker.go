// Package workers provides a bounded in-process worker pool with retry,
// tracing, and mandatory observability through webFramework.AddLog.
//
// Workers run outside HTTP request contexts, so each job receives a
// job-owned WebFramework backed by a concurrency-safe BackgroundParser.
// This ensures that webFramework.AddLog calls (including those from
// external API calls via handlers.CallAPI) flow into the Splunk
// transaction pipeline.
package workers

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// JobHandler is the function executed by a worker job.
// The JobContext provides a context and a job-owned WebFramework
// for mandatory AddLog calls.
type JobHandler func(*JobContext) error

// JobContext provides the execution context for a worker job.
type JobContext struct {
	// Context carries cancellation and tracing.
	Context context.Context

	// WebFramework is a job-owned WebFramework with a BackgroundParser
	// that supports webFramework.AddLog calls. This ensures that
	// external API calls and critical business events within jobs
	// are logged to the Splunk transaction pipeline.
	WebFramework any

	// JobName is the name of the job.
	JobName string

	// Attempt is the current attempt number (1-based).
	Attempt int

	// Attributes are tracing attributes for the job.
	Attributes map[string]string
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

// InProcessWorker is a bounded goroutine pool implementation of Worker.
type InProcessWorker struct {
	config   Config
	queue    chan Job
	wg       sync.WaitGroup
	shutdown atomic.Bool
	stats    Stats
	mu       sync.Mutex
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
		config: config,
		queue:  make(chan Job, config.QueueSize),
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
	for job := range w.queue {
		atomic.AddInt64(&w.stats.InFlight, 1)
		w.executeJob(job)
		atomic.AddInt64(&w.stats.InFlight, -1)
	}
}

func (w *InProcessWorker) executeJob(job Job) {
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

	var lastErr error
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		ctx := context.Background()
		if job.Options.Attributes == nil {
			job.Options.Attributes = make(map[string]string)
		}

		jobCtx := &JobContext{
			Context:    ctx,
			JobName:    job.Name,
			Attempt:    attempt,
			Attributes: job.Options.Attributes,
		}

		err := w.runWithObservability(jobCtx, job.Handler)
		if err == nil {
			atomic.AddInt64(&w.stats.Succeeded, 1)
			return
		}

		lastErr = err
		if attempt < opts.MaxAttempts {
			backoff := calculateBackoff(opts.InitialBackoff, opts.MaxBackoff, attempt, opts.Jitter)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
		}
	}

	atomic.AddInt64(&w.stats.Failed, 1)
	if opts.OnFailure != nil {
		opts.OnFailure(lastErr, opts.MaxAttempts)
	}
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

func (w *InProcessWorker) Submit(ctx context.Context, job Job) error {
	if w.shutdown.Load() {
		return ErrShutdown
	}

	atomic.AddInt64(&w.stats.Submitted, 1)

	if w.config.BlockOnFull {
		select {
		case w.queue <- job:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	select {
	case w.queue <- job:
		return nil
	default:
		return ErrQueueFull
	}
}

func (w *InProcessWorker) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	w.shutdown.Store(true)
	close(w.queue)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

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
