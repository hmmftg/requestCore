package remotecall

import "time"

// BackoffPolicy configures the delay between retry attempts. When non-nil
// and set on a RetryPolicy, the adapter translates it into a Resty
// RetryDelayStrategyFunc.
//
// If Multiplier is 0, it defaults to 2.0 (exponential).
// If JitterFactor is 0, no jitter is applied. Valid range is 0.0–1.0,
// where 0.2 means ±20% of the computed delay.
//
// For a fixed delay (no exponential growth, no jitter), set Multiplier=1
// and JitterFactor=0.
type BackoffPolicy struct {
	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay caps the computed delay.
	MaxDelay time.Duration

	// Multiplier is the exponential growth factor (default: 2.0).
	Multiplier float64

	// JitterFactor is the fraction of the delay to randomize (0.0–1.0).
	JitterFactor float64
}

// RetryPolicy contains declarative retry policy. The adapter translates
// this into Resty's retry conditions and execution configuration. The
// adapter does not independently invent retry decisions.
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

	// RetryOnBody is an optional callback that inspects the response
	// body to determine retry eligibility. It is called only when an
	// HTTP response was received (resp != nil). Use this for
	// application-level retry signals embedded in the response body
	// (e.g., Galaxy's ExceptionDetail.Key == "SERVICE_UNAVAILABLE").
	//
	// The callback receives the HTTP status code and the raw response
	// body bytes. Return true to retry, false to stop.
	RetryOnBody func(statusCode int, body []byte) bool

	// RetryOnTimeout controls whether requestCore-owned timeout errors
	// are retryable. This applies to requestCore-owned timeout only,
	// not caller deadline. The implementation must not retry a caller
	// deadline because errors.Is(err, context.DeadlineExceeded) is true.
	RetryOnTimeout bool

	// RetryOnTransport controls whether transport errors (DNS/TLS/network)
	// are retryable.
	RetryOnTransport bool

	// Backoff configures the delay between retry attempts. If nil,
	// the adapter uses a reasonable default (100ms initial, 2s max,
	// exponential with jitter via Resty's built-in strategy).
	Backoff *BackoffPolicy

	// HonorRetryAfter controls whether the Retry-After HTTP header is
	// honored for retried status codes. Resty's built-in behavior only
	// honors Retry-After for 429 and 503. When HonorRetryAfter is true,
	// the adapter extends this to all status codes in RetryOnStatus.
	//
	// This is useful for rate-limiting servers that return custom
	// status codes (e.g., 406, 330) with a Retry-After header.
	HonorRetryAfter bool
}
