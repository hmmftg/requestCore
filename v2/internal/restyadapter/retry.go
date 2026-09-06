package restyadapter

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/v2/remotecall"
	"resty.dev/v3"
)

// applyRetryPolicy translates a v2 RetryPolicy into Resty's per-request
// retry configuration. This is the sole place where Resty retry mechanics
// are configured.
//
// Resty's built-in retry defaults are disabled at the client level (see New),
// so they cannot cause unauthorized retries. This function is the sole
// authority on retry behavior.
func applyRetryPolicy(r *resty.Request, policy *remotecall.RetryPolicy) {
	if policy == nil {
		return
	}

	// MaxRetries = retries after initial attempt
	r.SetRetryCount(policy.MaxRetries)

	// Disable Resty's default retry conditions at the request level too,
	// as a belt-and-suspenders measure.
	r.SetRetryDefaultConditions(false)

	// RetryableMethods is the authoritative filter. If the request's method
	// is not in the map, set MaxRetries to 0 and return early — no retry
	// conditions are registered. The lookup is case-insensitive.
	methodAllowed := func() bool {
		if len(policy.RetryableMethods) == 0 {
			return false
		}
		upperMethod := strings.ToUpper(r.Method)
		// Fast path: exact uppercase match
		if policy.RetryableMethods[upperMethod] {
			return true
		}
		// Slow path: case-insensitive match for non-uppercase keys
		for m := range policy.RetryableMethods {
			if strings.EqualFold(m, r.Method) {
				return true
			}
		}
		return false
	}

	// If the method is not retryable, no retries.
	if !methodAllowed() {
		r.SetRetryCount(0)
		return
	}

	// Derive SetRetryAllowNonIdempotent from RetryableMethods.
	// If any non-idempotent method is in RetryableMethods, allow it.
	nonIdempotent := false
	for method := range policy.RetryableMethods {
		if !isIdempotentMethod(method) {
			nonIdempotent = true
			break
		}
	}
	if nonIdempotent {
		r.SetRetryAllowNonIdempotent(true)
	}

	// Configure backoff/delay strategy
	initialDelay := 100 * time.Millisecond
	maxDelay := 2 * time.Second
	multiplier := 2.0
	jitterFactor := 0.0

	if policy.Backoff != nil {
		if policy.Backoff.InitialDelay > 0 {
			initialDelay = policy.Backoff.InitialDelay
		}
		if policy.Backoff.MaxDelay > 0 {
			maxDelay = policy.Backoff.MaxDelay
		}
		if policy.Backoff.Multiplier > 0 {
			multiplier = policy.Backoff.Multiplier
		}
		if policy.Backoff.JitterFactor > 0 && policy.Backoff.JitterFactor <= 1.0 {
			jitterFactor = policy.Backoff.JitterFactor
		}
	}

	r.SetRetryWaitTime(initialDelay)
	r.SetRetryMaxWaitTime(maxDelay)

	// Build a custom delay strategy that implements exponential backoff
	// with optional jitter, and optionally honors Retry-After for custom
	// status codes.
	statusSet := make(map[int]bool, len(policy.RetryOnStatus))
	for code := range policy.RetryOnStatus {
		statusSet[code] = true
	}

	delayStrategy := buildDelayStrategy(
		initialDelay, maxDelay, multiplier, jitterFactor,
		policy.HonorRetryAfter, statusSet,
	)
	r.SetRetryDelayStrategy(delayStrategy)

	// Add retry condition for status codes
	if len(policy.RetryOnStatus) > 0 {
		r.AddRetryConditions(func(resp *resty.Response, err error) bool {
			if resp != nil && statusSet[resp.StatusCode()] {
				return true
			}
			return false
		})
	}

	// Add retry condition for body inspection
	if policy.RetryOnBody != nil {
		r.AddRetryConditions(func(resp *resty.Response, err error) bool {
			if resp == nil {
				return false
			}
			return policy.RetryOnBody(resp.StatusCode(), resp.Bytes())
		})
	}

	// Add retry condition for transport errors
	if policy.RetryOnTransport {
		r.AddRetryConditions(func(resp *resty.Response, err error) bool {
			if err != nil && resp == nil {
				return true
			}
			return false
		})
	}

	// Add retry condition for timeout errors
	if policy.RetryOnTimeout {
		r.AddRetryConditions(func(resp *resty.Response, err error) bool {
			if err == nil {
				return false
			}
			// Check for deadline exceeded — the adapter doesn't distinguish
			// caller vs requestCore timeout; that classification happens in
			// RemoteClient.Call. The adapter retries on any timeout if
			// RetryOnTimeout is set.
			return isTimeoutError(err)
		})
	}

	// Context cancellation is never retried — Resty breaks out of the retry
	// loop when context is done, so no explicit condition is needed.
}

// isIdempotentMethod reports whether the HTTP method is idempotent per
// RFC 9110 Section 9.2.2.
func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut,
		http.MethodDelete, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// isTimeoutError checks whether the error is a timeout/deadline error.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// buildDelayStrategy creates a Resty RetryDelayStrategyFunc that implements
// exponential backoff with optional jitter. When honorRetryAfter is true,
// the strategy checks the Retry-After header for any status code in
// retryStatusSet (extending Resty's built-in 429/503 behavior).
func buildDelayStrategy(
	initialDelay, maxDelay time.Duration,
	multiplier, jitterFactor float64,
	honorRetryAfter bool,
	retryStatusSet map[int]bool,
) resty.RetryDelayStrategyFunc {
	return func(resp *resty.Response, err error) (time.Duration, error) {
		// Honor Retry-After header for custom status codes when enabled.
		if honorRetryAfter && resp != nil {
			if retryStatusSet[resp.StatusCode()] {
				if delay, ok := parseRetryAfter(resp.Header().Get("Retry-After")); ok {
					return delay, nil
				}
			}
		}

		// Determine the attempt number (1-based from Resty).
		attempt := 1
		if resp != nil && resp.Request != nil {
			attempt = resp.Request.Attempt
		}
		if attempt < 1 {
			attempt = 1
		}

		// Compute exponential backoff: initialDelay * multiplier^(attempt-1)
		delay := float64(initialDelay) * math.Pow(multiplier, float64(attempt-1))
		if delay > float64(maxDelay) {
			delay = float64(maxDelay)
		}

		// Apply jitter: ±jitterFactor * delay
		if jitterFactor > 0 {
			jitterRange := delay * jitterFactor
			jitter := (rand.Float64()*2 - 1) * jitterRange
			delay += jitter
			if delay < 0 {
				delay = 0
			}
		}

		return time.Duration(delay), nil
	}
}

// parseRetryAfter parses the Retry-After header value. It supports both
// delta-seconds and HTTP-date formats per RFC 9110 Section 10.2.3.
func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	// Try as seconds (integer).
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	// Try as HTTP-date.
	if t, err := http.ParseTime(value); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d, true
		}
		return 0, false
	}

	return 0, false
}
