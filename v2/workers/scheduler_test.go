package workers

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/telemetry"
)

// TestScheduler_BasicTick verifies that a scheduled job's handler
// runs at the configured interval.
func TestScheduler_BasicTick(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	var ticks int64
	err := sched.Schedule(ScheduledJob{
		Name:     "test-tick",
		Interval: 20 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticks, 1)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()

	// Wait for at least 2 ticks.
	time.Sleep(60 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if atomic.LoadInt64(&ticks) < 2 {
		t.Fatalf("expected at least 2 ticks, got %d", atomic.LoadInt64(&ticks))
	}
}

// TestScheduler_GracefulShutdown verifies that Shutdown stops all
// tickers and waits for in-flight ticks to complete.
func TestScheduler_GracefulShutdown(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	var ticks int64
	err := sched.Schedule(ScheduledJob{
		Name:     "test-shutdown",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticks, 1)
			time.Sleep(20 * time.Millisecond) // simulate work
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()
	time.Sleep(30 * time.Millisecond) // let at least one tick start

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// After shutdown, no more ticks should occur.
	countAfter := atomic.LoadInt64(&ticks)
	time.Sleep(30 * time.Millisecond)
	if atomic.LoadInt64(&ticks) != countAfter {
		t.Fatalf("ticks continued after shutdown: before=%d, after=%d", countAfter, atomic.LoadInt64(&ticks))
	}
}

// TestScheduler_PanicRecovery verifies that a panic in the handler
// does not kill the scheduler goroutine.
func TestScheduler_PanicRecovery(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	var ticks int64
	err := sched.Schedule(ScheduledJob{
		Name:     "test-panic",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticks, 1)
			if atomic.LoadInt64(&ticks) == 1 {
				panic("boom")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// The scheduler should have continued ticking after the panic.
	if atomic.LoadInt64(&ticks) < 2 {
		t.Fatalf("expected scheduler to survive panic and tick again, got %d ticks", atomic.LoadInt64(&ticks))
	}

	// Verify the panic was recorded as a failure.
	stats := sched.Stats()
	if stats["test-panic"].Failed == 0 {
		t.Fatal("expected at least 1 failed tick from panic")
	}
}

// TestScheduler_MultipleJobs verifies that multiple scheduled jobs
// run independently.
func TestScheduler_MultipleJobs(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	var ticksA, ticksB int64
	if err := sched.Schedule(ScheduledJob{
		Name:     "job-a",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticksA, 1)
			return nil
		},
	}); err != nil {
		t.Fatalf("Schedule job-a: %v", err)
	}
	if err := sched.Schedule(ScheduledJob{
		Name:     "job-b",
		Interval: 15 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticksB, 1)
			return nil
		},
	}); err != nil {
		t.Fatalf("Schedule job-b: %v", err)
	}

	sched.Start()
	time.Sleep(50 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if atomic.LoadInt64(&ticksA) < 2 {
		t.Fatalf("job-a: expected at least 2 ticks, got %d", atomic.LoadInt64(&ticksA))
	}
	if atomic.LoadInt64(&ticksB) < 2 {
		t.Fatalf("job-b: expected at least 2 ticks, got %d", atomic.LoadInt64(&ticksB))
	}
}

// TestScheduler_Observability verifies that telemetry events are emitted
// per tick (the sink receives EventSuccess with the correct operation).
func TestScheduler_Observability(t *testing.T) {
	sink := &capturingSink{}
	sched := NewScheduler(SchedulerConfig{
		Logger: slog.New(slog.NewTextHandler(&discardHandler{}, &slog.HandlerOptions{Level: slog.LevelError})),
		Sink:   sink,
	})

	var ticks int64
	err := sched.Schedule(ScheduledJob{
		Name:     "test-obs",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			atomic.AddInt64(&ticks, 1)
			// The Logger should be non-nil for scoped logging.
			if ctx.Logger == nil {
				return errors.New("logger is nil")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()
	time.Sleep(40 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if atomic.LoadInt64(&ticks) < 1 {
		t.Fatalf("expected at least 1 tick, got %d", atomic.LoadInt64(&ticks))
	}

	// Verify stats show successful ticks.
	stats := sched.Stats()
	if stats["test-obs"].Succeeded == 0 {
		t.Fatal("expected at least 1 succeeded tick")
	}

	// Verify the telemetry sink received success events with the
	// correct operation name.
	events := sink.Events()
	if len(events) == 0 {
		t.Fatal("expected at least 1 telemetry event")
	}
	foundSuccess := false
	for _, ev := range events {
		if ev.Type == telemetry.EventSuccess && ev.Operation == "worker-test-obs-req" {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Fatalf("expected EventSuccess with operation 'worker-test-obs-req', got %d events", len(events))
	}
}

// TestScheduler_Stats verifies that Stats returns per-job statistics.
func TestScheduler_Stats(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	err := sched.Schedule(ScheduledJob{
		Name:     "test-stats",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()
	time.Sleep(30 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = sched.Shutdown(shutdownCtx)

	stats := sched.Stats()
	entry, ok := stats["test-stats"]
	if !ok {
		t.Fatal("expected stats for test-stats job")
	}
	if entry.Ticks == 0 {
		t.Fatal("expected at least 1 tick in stats")
	}
}

// TestScheduler_InvalidJob verifies that Schedule rejects invalid jobs.
func TestScheduler_InvalidJob(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	// Empty name.
	if err := sched.Schedule(ScheduledJob{
		Name:     "",
		Interval: 10 * time.Millisecond,
		Handler:  func(ctx *JobContext) error { return nil },
	}); err == nil {
		t.Fatal("expected error for empty name")
	}

	// Nil handler.
	if err := sched.Schedule(ScheduledJob{
		Name:     "test",
		Interval: 10 * time.Millisecond,
		Handler:  nil,
	}); err == nil {
		t.Fatal("expected error for nil handler")
	}

	// Zero interval.
	if err := sched.Schedule(ScheduledJob{
		Name:     "test",
		Interval: 0,
		Handler:  func(ctx *JobContext) error { return nil },
	}); err == nil {
		t.Fatal("expected error for zero interval")
	}
}

// TestScheduler_DuplicateName verifies that Schedule rejects duplicate
// job names.
func TestScheduler_DuplicateName(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	job := ScheduledJob{
		Name:     "dup",
		Interval: 10 * time.Millisecond,
		Handler:  func(ctx *JobContext) error { return nil },
	}
	if err := sched.Schedule(job); err != nil {
		t.Fatalf("first Schedule: %v", err)
	}
	if err := sched.Schedule(job); err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

// TestScheduler_ShutdownIdempotent verifies that Shutdown can be called
// multiple times safely.
func TestScheduler_ShutdownIdempotent(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	_ = sched.Schedule(ScheduledJob{
		Name:     "test-idem",
		Interval: 10 * time.Millisecond,
		Handler:  func(ctx *JobContext) error { return nil },
	})
	sched.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sched.Shutdown(ctx); err != nil {
		t.Fatalf("first Shutdown: %v", err)
	}
	if err := sched.Shutdown(ctx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}

// TestScheduler_ScheduleAfterShutdown verifies that Schedule returns
// ErrShutdown after Shutdown has been called.
func TestScheduler_ScheduleAfterShutdown(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})
	sched.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = sched.Shutdown(ctx)

	err := sched.Schedule(ScheduledJob{
		Name:     "after-shutdown",
		Interval: 10 * time.Millisecond,
		Handler:  func(ctx *JobContext) error { return nil },
	})
	if err != ErrShutdown {
		t.Fatalf("expected ErrShutdown, got %v", err)
	}
}

// TestScheduler_RetryOnFailure verifies that retry logic applies
// within a single tick when the handler returns an error.
func TestScheduler_RetryOnFailure(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{})

	var attempts int64
	var succeeded int64
	err := sched.Schedule(ScheduledJob{
		Name:     "test-retry",
		Interval: 10 * time.Millisecond,
		Handler: func(ctx *JobContext) error {
			a := atomic.AddInt64(&attempts, 1)
			if a < 3 {
				return errors.New("transient error")
			}
			atomic.AddInt64(&succeeded, 1)
			return nil
		},
		Options: JobOptions{
			MaxAttempts:    3,
			InitialBackoff: 5 * time.Millisecond,
			MaxBackoff:     20 * time.Millisecond,
		},
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	sched.Start()
	// Wait long enough for the first tick (fires after interval) plus
	// retries (5ms + ~10ms backoff = ~15ms). 200ms is generous.
	time.Sleep(200 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = sched.Shutdown(shutdownCtx)

	if atomic.LoadInt64(&attempts) < 3 {
		t.Fatalf("expected at least 3 attempts (retries), got %d", atomic.LoadInt64(&attempts))
	}
	if atomic.LoadInt64(&succeeded) < 1 {
		t.Fatal("expected at least 1 successful tick after retries")
	}
}
