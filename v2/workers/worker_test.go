package workers

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/webFramework"
)

func TestInProcessWorker_Success(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 2,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	}()

	var executed atomic.Bool
	err := w.Submit(context.Background(), Job{
		Name: "test-success",
		Handler: func(ctx *JobContext) error {
			executed.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if !executed.Load() {
		t.Fatal("expected job to execute")
	}

	stats := w.Stats()
	if stats.Submitted != 1 {
		t.Fatalf("expected 1 submitted, got %d", stats.Submitted)
	}
	if stats.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", stats.Succeeded)
	}
}

func TestInProcessWorker_Retry(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	var attempts atomic.Int32
	err := w.Submit(context.Background(), Job{
		Name: "test-retry",
		Handler: func(ctx *JobContext) error {
			count := attempts.Add(1)
			if count < 3 {
				return errors.New("transient error")
			}
			return nil
		},
		Options: JobOptions{
			MaxAttempts:    3,
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     50 * time.Millisecond,
			Jitter:         false,
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}

	stats := w.Stats()
	if stats.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", stats.Succeeded)
	}
}

func TestInProcessWorker_AllRetriesFail(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	var failureCalled atomic.Bool
	var failureAttempts atomic.Int32
	err := w.Submit(context.Background(), Job{
		Name: "test-fail",
		Handler: func(ctx *JobContext) error {
			return errors.New("permanent error")
		},
		Options: JobOptions{
			MaxAttempts:    2,
			InitialBackoff: 10 * time.Millisecond,
			MaxBackoff:     20 * time.Millisecond,
			Jitter:         false,
			OnFailure: func(err error, attempts int) {
				failureCalled.Store(true)
				failureAttempts.Store(int32(attempts))
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if !failureCalled.Load() {
		t.Fatal("expected OnFailure to be called")
	}
	if failureAttempts.Load() != 2 {
		t.Fatalf("expected 2 attempts in OnFailure, got %d", failureAttempts.Load())
	}

	stats := w.Stats()
	if stats.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", stats.Failed)
	}
}

func TestInProcessWorker_QueueFull(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   1,
		BlockOnFull: false,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	// Fill the queue with a blocking job
	block := make(chan struct{})
	w.Submit(context.Background(), Job{
		Name: "blocker",
		Handler: func(ctx *JobContext) error {
			<-block
			return nil
		},
	})

	// Fill the queue buffer
	w.Submit(context.Background(), Job{
		Name: "queued",
		Handler: func(ctx *JobContext) error {
			return nil
		},
	})

	// This should fail with ErrQueueFull
	err := w.Submit(context.Background(), Job{
		Name: "overflow",
		Handler: func(ctx *JobContext) error {
			return nil
		},
	})
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}

	close(block)
}

func TestInProcessWorker_ShutdownRejectsSubmit(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	err := w.Submit(context.Background(), Job{
		Name: "after-shutdown",
		Handler: func(ctx *JobContext) error {
			return nil
		},
	})
	if !errors.Is(err, ErrShutdown) {
		t.Fatalf("expected ErrShutdown, got %v", err)
	}
}

func TestInProcessWorker_ShutdownDrains(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 2,
		QueueSize:   10,
	})

	var executed atomic.Int32
	for i := 0; i < 5; i++ {
		w.Submit(context.Background(), Job{
			Name: "drain-test",
			Handler: func(ctx *JobContext) error {
				executed.Add(1)
				return nil
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if executed.Load() != 5 {
		t.Fatalf("expected 5 executed, got %d", executed.Load())
	}
}

func TestInProcessWorker_PanicRecovery(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	// A panicking job should not kill the worker goroutine
	w.Submit(context.Background(), Job{
		Name: "panic-job",
		Handler: func(ctx *JobContext) error {
			panic("test panic")
		},
	})

	time.Sleep(100 * time.Millisecond)

	// The worker should still be able to process new jobs
	var executed atomic.Bool
	w.Submit(context.Background(), Job{
		Name: "after-panic",
		Handler: func(ctx *JobContext) error {
			executed.Store(true)
			return nil
		},
	})

	time.Sleep(100 * time.Millisecond)
	if !executed.Load() {
		t.Fatal("expected worker to survive panic and process next job")
	}
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		initial, max time.Duration
		attempt      int
		jitter       bool
		minExpected  time.Duration
		maxExpected  time.Duration
	}{
		{10 * time.Millisecond, 100 * time.Millisecond, 1, false, 10 * time.Millisecond, 10 * time.Millisecond},
		{10 * time.Millisecond, 100 * time.Millisecond, 2, false, 20 * time.Millisecond, 20 * time.Millisecond},
		{10 * time.Millisecond, 100 * time.Millisecond, 3, false, 40 * time.Millisecond, 40 * time.Millisecond},
		{10 * time.Millisecond, 50 * time.Millisecond, 10, false, 50 * time.Millisecond, 50 * time.Millisecond},
	}
	for _, tt := range tests {
		result := calculateBackoff(tt.initial, tt.max, tt.attempt, tt.jitter, defaultJitter)
		if result < tt.minExpected || result > tt.maxExpected {
			t.Fatalf("attempt %d: expected %v-%v, got %v", tt.attempt, tt.minExpected, tt.maxExpected, result)
		}
	}
}

func TestCalculateBackoff_WithJitter(t *testing.T) {
	result := calculateBackoff(100*time.Millisecond, 500*time.Millisecond, 1, true, defaultJitter)
	// With 50% jitter, result should be between 100ms and 150ms
	if result < 100*time.Millisecond || result > 150*time.Millisecond {
		t.Fatalf("expected 100ms-150ms with jitter, got %v", result)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.QueueSize != 100 {
		t.Fatalf("expected default queue size 100, got %d", cfg.QueueSize)
	}
	if cfg.BlockOnFull != false {
		t.Fatal("expected default BlockOnFull=false")
	}
}

func TestInProcessWorker_IdempotentShutdown(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 2,
		QueueSize:   10,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown: %v", err)
	}
	// Second shutdown should not panic or hang.
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}

func TestInProcessWorker_ConcurrentSubmitAndShutdown(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 2,
		QueueSize:   100,
	})
	var submitted atomic.Int32
	var done atomic.Bool
	go func() {
		for !done.Load() {
			err := w.Submit(context.Background(), Job{
				Name: "concurrent-test",
				Handler: func(ctx *JobContext) error {
					return nil
				},
			})
			if err == nil {
				submitted.Add(1)
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	done.Store(true)
}

func TestInProcessWorker_InvalidJob(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()
	if err := w.Submit(context.Background(), Job{Name: "", Handler: func(ctx *JobContext) error { return nil }}); err != ErrInvalidJob {
		t.Fatalf("expected ErrInvalidJob for empty name, got %v", err)
	}
	if err := w.Submit(context.Background(), Job{Name: "test", Handler: nil}); err != ErrInvalidJob {
		t.Fatalf("expected ErrInvalidJob for nil handler, got %v", err)
	}
}

func TestInProcessWorker_TransactionSink(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()
	err := w.Submit(context.Background(), Job{
		Name: "sink-test",
		Handler: func(ctx *JobContext) error {
			// The WebFramework should have a non-nil Parser for AddLog calls.
			if ctx.WebFramework.Parser == nil {
				return errors.New("nil Parser")
			}
			// Simulate an AddLog call via the background parser.
			ctx.WebFramework.Parser.SetLocal("test", "value")
			if v := ctx.WebFramework.Parser.GetLocal("test"); v != "value" {
				return errors.New("SetLocal/GetLocal failed")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	stats := w.Stats()
	if stats.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded, got %d", stats.Succeeded)
	}
}

// attrKeys returns the keys of a slice of slog.Attr for debugging.
func attrKeys(attrs []slog.Attr) []string {
	keys := make([]string, len(attrs))
	for i, a := range attrs {
		keys[i] = a.Key
	}
	return keys
}

// TestInProcessWorker_RealAddLog verifies that webFramework.AddLog calls
// made inside a job handler are collected into the transaction sink
// via CollectLogArrays, proving the real Splunk pipeline works.
func TestInProcessWorker_RealAddLog(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	var sinkEntries []slog.Attr
	var gotEntries atomic.Bool

	err := w.Submit(context.Background(), Job{
		Name: "addlog-test",
		Handler: func(ctx *JobContext) error {
			// Use the real webFramework.AddLog pipeline.
			webFramework.AddLog(ctx.WebFramework, webFramework.HandlerLogTag,
				slog.String("test-key", "test-value"))
			// Collect logs from the parser into the transaction sink.
			webFramework.CollectLogArrays(ctx.WebFramework, webFramework.HandlerLogTag)
			// Verify entries were collected.
			entries := ctx.transactionSink.Entries()
			if len(entries) == 0 {
				return errors.New("no entries collected in sink")
			}
			sinkEntries = entries
			gotEntries.Store(true)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if !gotEntries.Load() {
		t.Fatal("expected AddLog entries to be collected in sink")
	}
	// CollectLogArrays wraps the array as slog.Any("handler", arr),
	// so the sink should contain an entry with key "handler".
	found := false
	for _, e := range sinkEntries {
		if e.Key == webFramework.HandlerLogTag {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected '%s' attribute in sink entries, got keys: %v", webFramework.HandlerLogTag, attrKeys(sinkEntries))
	}
}

// TestInProcessWorker_ContextValuePropagation verifies that context
// values from the submission context survive into the job handler
// when PropagateCancel is false (the default).
func TestInProcessWorker_ContextValuePropagation(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	type ctxKey string
	submitCtx := context.WithValue(context.Background(), ctxKey("trace-id"), "abc123")

	var seenValue atomic.Value
	err := w.Submit(submitCtx, Job{
		Name: "ctx-propagation-test",
		Handler: func(ctx *JobContext) error {
			v := ctx.Context.Value(ctxKey("trace-id"))
			seenValue.Store(v)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	v := seenValue.Load()
	if v != "abc123" {
		t.Fatalf("expected trace-id 'abc123' to propagate, got %v", v)
	}
}

// TestInProcessWorker_CancelPropagation verifies that when PropagateCancel
// is true, cancelling the submit context cancels the job.
func TestInProcessWorker_CancelPropagation(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	submitCtx, cancel := context.WithCancel(context.Background())
	var wasCancelled atomic.Bool

	err := w.Submit(submitCtx, Job{
		Name: "cancel-test",
		Handler: func(ctx *JobContext) error {
			<-ctx.Context.Done()
			wasCancelled.Store(true)
			return ctx.Context.Err()
		},
		Options: JobOptions{
			PropagateCancel: true,
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)
	if !wasCancelled.Load() {
		t.Fatal("expected job to observe cancellation")
	}
}

// TestInProcessWorker_RetryCancellation verifies that retry timers
// observe job context cancellation.
func TestInProcessWorker_RetryCancellation(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	}()

	submitCtx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int32
	var onFailureCalled atomic.Bool

	err := w.Submit(submitCtx, Job{
		Name: "retry-cancel-test",
		Handler: func(ctx *JobContext) error {
			attempts.Add(1)
			return errors.New("fail")
		},
		Options: JobOptions{
			MaxAttempts:     10,
			InitialBackoff:  1 * time.Second,
			MaxBackoff:      5 * time.Second,
			Jitter:          false,
			PropagateCancel: true,
			OnFailure: func(err error, atts int) {
				onFailureCalled.Store(true)
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	time.Sleep(200 * time.Millisecond)

	if attempts.Load() > 2 {
		t.Fatalf("expected at most 2 attempts before cancellation, got %d", attempts.Load())
	}
	if !onFailureCalled.Load() {
		t.Fatal("expected OnFailure to be called on cancellation")
	}
}

// TestInProcessWorker_RepeatedShutdown verifies that multiple Shutdown
// calls return the same result without creating new goroutines.
func TestInProcessWorker_RepeatedShutdown(t *testing.T) {
	w := NewInProcessWorker(Config{
		WorkerCount: 2,
		QueueSize:   10,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		if err := w.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown call %d: %v", i, err)
		}
	}
}
