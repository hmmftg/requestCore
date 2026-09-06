package remotecall

import (
	"strings"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // allowing requests
	CircuitOpen                          // rejecting requests
	CircuitHalfOpen                      // allowing limited probes
)

// String returns a human-readable name for the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerPolicy configures a circuit breaker. Zero values are
// normalized to defaults by WithCircuitBreaker.
type CircuitBreakerPolicy struct {
	// MaxRequests is the half-open probe limit (default: 1).
	MaxRequests uint32

	// Interval is the closed-state reset cycle (default: 60s).
	Interval time.Duration

	// OpenDuration is the open-state duration before half-open (default: 60s).
	// Named OpenDuration, not Timeout, to avoid confusion with HTTP timeout.
	OpenDuration time.Duration

	// FailureThreshold is the number of consecutive failures to trip
	// from Closed to Open (default: 5).
	FailureThreshold uint32
}

// normalize fills zero values with defaults.
func (p *CircuitBreakerPolicy) normalize() {
	if p.MaxRequests == 0 {
		p.MaxRequests = 1
	}
	if p.Interval == 0 {
		p.Interval = 60 * time.Second
	}
	if p.OpenDuration == 0 {
		p.OpenDuration = 60 * time.Second
	}
	if p.FailureThreshold == 0 {
		p.FailureThreshold = 5
	}
}

// CircuitCounts tracks breaker statistics within a state.
type CircuitCounts struct {
	Failures      uint32
	Successes     uint32
	HalfOpenProbes uint32 // reserved probes in half-open
}

// CircuitBreaker is a per-API+method state machine. One breaker instance
// covers all calls to the same api.Name with the same HTTP method,
// regardless of path.
//
// Breaker state-machine semantics:
//   - Closed: failure count < FailureThreshold → Allow. failure count >=
//     FailureThreshold → transition to Open.
//   - Open: before OpenDuration expires → reject. After OpenDuration
//     expires → transition to HalfOpen.
//   - HalfOpen: up to MaxRequests concurrent calls may pass Allow().
//     Allow() atomically reserves a half-open probe slot. Success →
//     transition to Closed. Failure → transition to Open.
//
// Counter reset semantics:
//   - Entering Closed: resets all failure counters.
//   - Entering Open: starts the open timer, resets failure counters.
//   - Entering HalfOpen: resets the half-open request counter, retains no
//     closed-state failure history.
//   - Successful half-open probe: closes the breaker and resets all failure
//     counters.
//   - Failed half-open probe: reopens the breaker and restarts the open timer.
//
// Circuit breaking is opt-in. If WithCircuitBreaker is not called, no
// circuit breaker is active.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	maxRequests      uint32
	interval         time.Duration
	openDuration     time.Duration
	failureThreshold uint32
	counts           CircuitCounts
	openedAt         time.Time
}

// NewCircuitBreaker creates a breaker from a normalized policy.
func NewCircuitBreaker(policy CircuitBreakerPolicy) *CircuitBreaker {
	policy.normalize()
	return &CircuitBreaker{
		state:            CircuitClosed,
		maxRequests:      policy.MaxRequests,
		interval:         policy.Interval,
		openDuration:     policy.OpenDuration,
		failureThreshold: policy.FailureThreshold,
	}
}

// Allow returns true if a request is permitted. In HalfOpen, it atomically
// reserves a half-open probe slot. At most MaxRequests concurrent logical
// calls may pass Allow() before an outcome is recorded.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true

	case CircuitOpen:
		if time.Since(cb.openedAt) >= cb.openDuration {
			cb.state = CircuitHalfOpen
			cb.counts = CircuitCounts{}
			if cb.counts.HalfOpenProbes < cb.maxRequests {
				cb.counts.HalfOpenProbes++
				return true
			}
			return false
		}
		return false

	case CircuitHalfOpen:
		if cb.counts.HalfOpenProbes < cb.maxRequests {
			cb.counts.HalfOpenProbes++
			return true
		}
		return false

	default:
		return false
	}
}

// RecordSuccess records a successful outcome. In HalfOpen, a successful
// probe closes the breaker and resets all failure counters.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitHalfOpen:
		cb.state = CircuitClosed
		cb.counts = CircuitCounts{}
	case CircuitClosed:
		cb.counts.Failures = 0
	}
}

// RecordFailure records a failure outcome. In Closed, reaching
// FailureThreshold transitions to Open. In HalfOpen, a failure reopens
// the breaker and restarts the open timer.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		cb.counts.Failures++
		if cb.counts.Failures >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.openedAt = time.Now()
			cb.counts = CircuitCounts{}
		}

	case CircuitHalfOpen:
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		cb.counts = CircuitCounts{}
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// OperationKey is a string identifying a unique API+method combination
// for circuit breaker scoping.
type OperationKey string

// NewOperationKey constructs an OperationKey from an API name and HTTP method.
// The key is defined as apiName + ":" + strings.ToUpper(method).
// Example: NewOperationKey("payment-api", "post") → "payment-api:POST".
// The key is case-insensitive on method (normalized to uppercase).
func NewOperationKey(apiName, method string) OperationKey {
	return OperationKey(apiName + ":" + strings.ToUpper(method))
}

// String returns the string representation of the operation key.
func (k OperationKey) String() string {
	return string(k)
}
