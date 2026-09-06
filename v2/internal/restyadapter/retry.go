package restyadapter

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/v2/remotecall"
	"resty.dev/v3"
)

// applyRetryPolicy translates a v2 RetryPolicy into Resty's per-request
// retry configuration. This is the sole place where Resty retry mechanics
// are configured. The policy is declarative-only — no backoff/delay/jitter
// functions from v2.
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
	// conditions are registered.
	methodAllowed := func() bool {
		if len(policy.RetryableMethods) == 0 {
			return false
		}
		upperMethod := strings.ToUpper(r.Method)
		return policy.RetryableMethods[upperMethod]
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

	// Set reasonable default wait times (can be overridden by caller in future)
	r.SetRetryWaitTime(100 * time.Millisecond)
	r.SetRetryMaxWaitTime(2 * time.Second)

	// Add retry condition for status codes
	if len(policy.RetryOnStatus) > 0 {
		statusSet := make(map[int]bool, len(policy.RetryOnStatus))
		for code := range policy.RetryOnStatus {
			statusSet[code] = true
		}
		r.AddRetryConditions(func(resp *resty.Response, err error) bool {
			if resp != nil && statusSet[resp.StatusCode()] {
				return true
			}
			return false
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
