package workers

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
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
		WorkerCount:   1,
		QueueSize:     1,
		BlockOnFull:   false,
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
		result := calculateBackoff(tt.initial, tt.max, tt.attempt, tt.jitter)
		if result < tt.minExpected || result > tt.maxExpected {
			t.Fatalf("attempt %d: expected %v-%v, got %v", tt.attempt, tt.minExpected, tt.maxExpected, result)
		}
	}
}

func TestCalculateBackoff_WithJitter(t *testing.T) {
	result := calculateBackoff(100*time.Millisecond, 500*time.Millisecond, 1, true)
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
