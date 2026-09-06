package remotecall

// RetryPolicy contains only declarative retry policy — no execution mechanics.
//
// requestCore defines the retry policy; the adapter translates that policy
// into Resty's retry conditions and execution configuration. The adapter
// does not independently invent retry decisions.
//
// Retry classification answers: "Should I make another HTTP attempt?"
// This is independent from circuit-breaker classification, which answers:
// "Did this logical call provide evidence that the dependency is unhealthy?"
//
// For example:
//   - HTTP 503: retryable=yes, breaker failure=yes
//   - HTTP 429: retryable=potentially yes, breaker failure=no (success)
//   - HTTP 400: retryable=no, breaker failure=no (success)
//   - Transport timeout: retryable=RetryOnTimeout, breaker failure=yes
//   - context.Canceled: retryable=never, breaker failure=no outcome
type RetryPolicy struct {
	// MaxRetries is the number of retries after the initial attempt.
	// MaxRetries=2 → 3 total attempts (1 initial + 2 retries).
	// MaxRetries=0 → 1 attempt only (no retry).
	MaxRetries int

	// RetryableMethods specifies which HTTP methods are retryable.
	// The adapter derives SetRetryAllowNonIdempotent(true) when this
	// map contains any non-idempotent method (POST, PATCH, etc.).
	// RetryableMethods is the authoritative filter.
	RetryableMethods map[string]bool

	// RetryOnStatus specifies which HTTP status codes trigger retry.
	// This is declarative (map[int]bool), not a callback.
	RetryOnStatus map[int]bool

	// RetryOnTimeout controls whether requestCore-owned timeout errors
	// are retryable. This applies to requestCore-owned timeout only,
	// not caller deadline. The implementation must not retry a caller
	// deadline because errors.Is(err, context.DeadlineExceeded) is true.
	RetryOnTimeout bool

	// RetryOnTransport controls whether transport errors (DNS/TLS/network)
	// are retryable.
	RetryOnTransport bool
}

// No `Backoff`, `DelayStrategy`, `Jitter`, or `RetryAfterParser` functions
// in the public policy. Those are Resty execution mechanics, not requestCore
// policy. No `AllowNonIdempotent` field — derived from RetryableMethods.
