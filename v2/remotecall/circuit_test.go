package remotecall_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/remotecall"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{
		MaxRequests:      1,
		Interval:         60 * time.Second,
		OpenDuration:     50 * time.Millisecond,
		FailureThreshold: 3,
	})

	if cb.State() != remotecall.CircuitClosed {
		t.Fatalf("expected closed, got %s", cb.State())
	}

	// Record failures to trip the breaker
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatal("expected Allow to return true while closed")
		}
		cb.RecordFailure()
	}

	if cb.State() != remotecall.CircuitOpen {
		t.Fatalf("expected open after 3 failures, got %s", cb.State())
	}

	// While open, Allow should reject
	if cb.Allow() {
		t.Fatal("expected Allow to return false while open")
	}

	// Wait for open duration to expire
	time.Sleep(60 * time.Millisecond)

	// Now Allow should transition to half-open and accept one probe
	if !cb.Allow() {
		t.Fatal("expected Allow to return true in half-open")
	}
	if cb.State() != remotecall.CircuitHalfOpen {
		t.Fatalf("expected half-open, got %s", cb.State())
	}

	// Second probe should be rejected (MaxRequests=1)
	if cb.Allow() {
		t.Fatal("expected Allow to return false for second half-open probe")
	}

	// Successful probe closes the breaker
	cb.RecordSuccess()
	if cb.State() != remotecall.CircuitClosed {
		t.Fatalf("expected closed after successful probe, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{
		MaxRequests:      1,
		OpenDuration:     50 * time.Millisecond,
		FailureThreshold: 1,
	})

	// Trip the breaker
	cb.Allow()
	cb.RecordFailure()
	if cb.State() != remotecall.CircuitOpen {
		t.Fatalf("expected open, got %s", cb.State())
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("expected Allow in half-open")
	}

	// Failure in half-open reopens
	cb.RecordFailure()
	if cb.State() != remotecall.CircuitOpen {
		t.Fatalf("expected open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_ClosedSuccessResetsFailures(t *testing.T) {
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{
		FailureThreshold: 3,
	})

	// Two failures
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	// Success resets failure count
	cb.Allow()
	cb.RecordSuccess()

	// Two more failures should not trip (counter was reset)
	cb.Allow()
	cb.RecordFailure()
	cb.Allow()
	cb.RecordFailure()

	if cb.State() != remotecall.CircuitClosed {
		t.Fatalf("expected still closed after reset+2 failures, got %s", cb.State())
	}
}

func TestCircuitBreaker_ZeroValueNormalization(t *testing.T) {
	// Zero-value policy should normalize to defaults
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{})

	// Should not panic and should be in closed state
	if cb.State() != remotecall.CircuitClosed {
		t.Fatalf("expected closed, got %s", cb.State())
	}

	// Should be able to Allow
	if !cb.Allow() {
		t.Fatal("expected Allow to return true")
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{
		MaxRequests:      10,
		OpenDuration:     50 * time.Millisecond,
		FailureThreshold: 100, // high to prevent tripping during test
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Allow()
			cb.RecordSuccess()
		}()
	}
	wg.Wait()

	if cb.State() != remotecall.CircuitClosed {
		t.Fatalf("expected closed after concurrent successes, got %s", cb.State())
	}
}

func TestOperationKey_Deterministic(t *testing.T) {
	k1 := remotecall.NewOperationKey("payment-api", "post")
	k2 := remotecall.NewOperationKey("payment-api", "POST")
	k3 := remotecall.NewOperationKey("payment-api", "Post")

	if k1 != k2 {
		t.Errorf("expected same key for post and POST, got %q vs %q", k1, k2)
	}
	if k1 != k3 {
		t.Errorf("expected same key for post and Post, got %q vs %q", k1, k3)
	}

	k4 := remotecall.NewOperationKey("payment-api", "get")
	if k1 == k4 {
		t.Error("expected different keys for POST and GET")
	}

	expected := "payment-api:POST"
	if k1.String() != expected {
		t.Errorf("expected %q, got %q", expected, k1.String())
	}
}

func TestCircuitBreaker_DifferentOperationKeysIsolated(t *testing.T) {
	// Verify that different operation keys create isolated breakers
	// (This is tested through RemoteClient, but we verify the key isolation here)
	k1 := remotecall.NewOperationKey("api1", "get")
	k2 := remotecall.NewOperationKey("api2", "get")
	k3 := remotecall.NewOperationKey("api1", "post")

	if k1 == k2 || k1 == k3 || k2 == k3 {
		t.Error("expected different operation keys to be isolated")
	}
}

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    remotecall.CircuitState
		expected string
	}{
		{remotecall.CircuitClosed, "closed"},
		{remotecall.CircuitOpen, "open"},
		{remotecall.CircuitHalfOpen, "half-open"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("CircuitState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestCircuitBreaker_HalfOpenConcurrency(t *testing.T) {
	cb := remotecall.NewCircuitBreaker(remotecall.CircuitBreakerPolicy{
		MaxRequests:      3,
		OpenDuration:     50 * time.Millisecond,
		FailureThreshold: 1,
	})

	// Trip the breaker
	cb.Allow()
	cb.RecordFailure()

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Concurrent Allow calls — at most MaxRequests should pass
	var wg sync.WaitGroup
	allowed := make([]bool, 10)
	var mu sync.Mutex
	allowedCount := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if cb.Allow() {
				mu.Lock()
				allowed[idx] = true
				allowedCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if allowedCount > 3 {
		t.Errorf("expected at most %d allowed, got %d", 3, allowedCount)
	}
	fmt.Printf("half-open concurrent: %d allowed out of 10\n", allowedCount)
}
