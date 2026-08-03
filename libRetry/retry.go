package libRetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
)

// StatusProvider is an optional interface that response types can implement
// to expose their HTTP status code for retry eligibility checks.
type StatusProvider interface {
	GetStatus() int
}

// ErrorCodeProvider is an optional interface that response types can implement
// to expose an application-level error code for retry eligibility checks.
type ErrorCodeProvider interface {
	GetErrorCode() int
}

// RetryPolicy configures the retry behavior for a sequence of attempts.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts (0 = no retries,
	// total attempts = MaxRetries + 1).
	MaxRetries int

	// RetryOnTimeout enables retrying when a timeout error is detected.
	RetryOnTimeout bool

	// RetryOnStatus is a set of HTTP status codes that should trigger a retry.
	RetryOnStatus map[int]bool

	// RetryOnErrorCodes is a set of application-level error codes (from
	// ErrorCodeProvider) that should trigger a retry.
	RetryOnErrorCodes map[int]bool

	// Backoff is the duration to wait between retry attempts.
	// If zero, no delay is applied between attempts.
	Backoff time.Duration

	// IsTimeoutError is an optional predicate to determine if an error is a
	// timeout error. If nil, the default predicate is used, which recognizes
	// API_CONNECT_TIMED_OUT and API_CALL_TIME_OUT error descriptions.
	IsTimeoutError func(err error) bool

	// Context for cancellation. If nil, context.Background() is used.
	Context context.Context

	// Sleep is an optional function used for backoff delays. If nil,
	// a timer-based implementation is used that exits early on context
	// cancellation. Useful for deterministic testing.
	Sleep func(ctx context.Context, d time.Duration) bool
}

// RetryResult holds the outcome of a retry sequence.
type RetryResult[Resp any] struct {
	Response   *Resp
	Error      error
	Elapsed    time.Duration
	Attempts   int
	LastStatus int
}

// AttemptFunc is the function executed for each retry attempt.
// The attempt number (1-based) is passed as the title argument.
// It returns the response, an error, and the HTTP status code observed.
type AttemptFunc[Resp any] func(attempt int) (*Resp, error, int)

// defaultIsTimeoutError checks whether an error represents a timeout condition
// by looking for known libError descriptions.
func defaultIsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if ok, libErr := libError.Unwrap(err); ok {
		desc := libErr.Action().Description
		if desc == "API_CONNECT_TIMED_OUT" || desc == "API_CALL_TIME_OUT" {
			return true
		}
	}
	// Also check for the handlers.ErrServerTimeout sentinel via errors.Is
	// (import cycle prevents direct reference; check by error message)
	return strings.Contains(err.Error(), "server-side timeout exceeded")
}

// defaultSleep waits for duration d, returning false if the context is
// cancelled before the timer fires.
func defaultSleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// WithRetry executes the given attempt function up to MaxRetries + 1 times,
// retrying based on the policy configuration. It returns the final result
// with aggregate metadata.
func WithRetry[Resp any](policy *RetryPolicy, attempt AttemptFunc[Resp]) RetryResult[Resp] {
	result := RetryResult[Resp]{}

	if policy == nil {
		policy = &RetryPolicy{}
	}

	ctx := policy.Context
	if ctx == nil {
		ctx = context.Background()
	}

	sleepFn := policy.Sleep
	if sleepFn == nil {
		sleepFn = defaultSleep
	}

	isTimeout := policy.IsTimeoutError
	if isTimeout == nil {
		isTimeout = defaultIsTimeoutError
	}

	maxAttempts := policy.MaxRetries + 1
	start := time.Now()

	for attemptNum := 1; attemptNum <= maxAttempts; attemptNum++ {
		result.Attempts = attemptNum

		resp, err, status := attempt(attemptNum)
		result.LastStatus = status
		result.Response = resp
		result.Error = err
		result.Elapsed = time.Since(start)

		if err == nil {
			// Success — but check if the response indicates a retryable status
			if shouldRetryResponse(policy, resp, status) && attemptNum < maxAttempts {
				if !sleepFn(ctx, policy.Backoff) {
					// Context cancelled during backoff
					result.Error = ctx.Err()
					return result
				}
				continue
			}
			return result
		}

		// Error case — check if we should retry
		if attemptNum >= maxAttempts {
			return result
		}

		if !shouldRetryError(policy, err, status, isTimeout) {
			return result
		}

		// Backoff before next attempt
		if !sleepFn(ctx, policy.Backoff) {
			result.Error = ctx.Err()
			return result
		}
	}

	return result
}

// shouldRetryResponse checks if a successful response should be retried
// based on status code or error code in the response.
func shouldRetryResponse[Resp any](policy *RetryPolicy, resp *Resp, status int) bool {
	if resp == nil {
		return false
	}

	// Check HTTP status against RetryOnStatus
	if policy.RetryOnStatus != nil && status > 0 && policy.RetryOnStatus[status] {
		return true
	}

	// Check response's StatusProvider interface
	if sp, ok := any(resp).(StatusProvider); ok {
		s := sp.GetStatus()
		if policy.RetryOnStatus != nil && policy.RetryOnStatus[s] {
			return true
		}
	}

	// Check response's ErrorCodeProvider interface
	if ecp, ok := any(resp).(ErrorCodeProvider); ok {
		code := ecp.GetErrorCode()
		if policy.RetryOnErrorCodes != nil && policy.RetryOnErrorCodes[code] {
			return true
		}
	}

	return false
}

// shouldRetryError checks if an error response should be retried.
func shouldRetryError(policy *RetryPolicy, err error, status int, isTimeout func(error) bool) bool {
	// Check timeout retry
	if policy.RetryOnTimeout && isTimeout(err) {
		return true
	}

	// Check RemoteCallError status
	var rce *libCallApi.RemoteCallError
	if errors.As(err, &rce) {
		if policy.RetryOnStatus != nil && policy.RetryOnStatus[rce.Status] {
			return true
		}
	}

	// Check status from attempt function
	if policy.RetryOnStatus != nil && status > 0 && policy.RetryOnStatus[status] {
		return true
	}

	return false
}

// FormatAttemptTitle returns a log key suffix for the given attempt number.
// Attempt 1 has no suffix; attempts 2+ get "-retry-N".
func FormatAttemptTitle(base string, attempt int) string {
	if attempt <= 1 {
		return base
	}
	return fmt.Sprintf("%s-retry-%d", base, attempt-1)
}
