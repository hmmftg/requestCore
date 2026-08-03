package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

// ErrServerTimeout is the sentinel error wrapped inside BuildTimeoutError.
// Use errors.Is(err, handlers.ErrServerTimeout) to check for server-side
// timeout errors regardless of additional context.
var ErrServerTimeout = errors.New("server-side timeout exceeded")

// BuildTimeoutError creates a standardized libError for server-side timeout
// conditions. The domain is included for context. The returned error wraps
// ErrServerTimeout so callers can use errors.Is to detect timeout errors.
func BuildTimeoutError(domain string) error {
	timeoutErr := libError.NewWithDescription(
		status.StatusCode(http.StatusRequestTimeout),
		"API_CALL_TIME_OUT",
		"%s: elapsed time exceeds timeout: %s",
		domain, ErrServerTimeout.Error(),
	)
	return errors.Join(timeoutErr, ErrServerTimeout)
}

// NormalizeCallError converts a raw remote-call error into a standardized
// libError with a consistent API_CALL_ERROR description. It uses libError.Unwrap
// to detect typed errors and preserves specific error codes that require
// special handling (e.g., API_CONNECT_TIMED_OUT for retry predicates).
//
// Behavior:
//   - nil → nil
//   - API_CONNECT_TIMED_OUT: returned unchanged (retry predicates depend on it)
//   - API_UNABLE_PARSE_RESP: re-wrapped with useful child text if available
//   - Other libError values: re-wrapped as API_CALL_ERROR with original context
//   - Non-libError errors: returned unchanged
func NormalizeCallError(err error) error {
	if err == nil {
		return nil
	}

	// Handle RemoteCallError (not a libError type)
	var rce *libCallApi.RemoteCallError
	if errors.As(err, &rce) {
		return libError.NewWithDescription(
			status.StatusCode(rce.Status),
			"API_CALL_ERROR",
			"HTTP %d: %s",
			rce.Status, string(rce.Body),
		)
	}

	ok, libErr := libError.Unwrap(err)
	if !ok {
		return err
	}

	desc := libErr.Action().Description

	switch desc {
	case "API_CONNECT_TIMED_OUT":
		// Preserve unchanged so retry predicates can identify timeout errors
		return err

	case "API_UNABLE_PARSE_RESP":
		// Try to extract useful child error text from the formatted error
		errStr := err.Error()
		if idx := strings.Index(errStr, "\n"); idx >= 0 {
			childText := strings.TrimSpace(errStr[idx+1:])
			if childText != "" {
				return libError.NewWithDescription(
					status.InternalServerError,
					"API_CALL_ERROR",
					"parse response failed: %s (original: %s)",
					childText, desc,
				)
			}
		}
		return libError.NewWithDescription(
			status.InternalServerError,
			"API_CALL_ERROR",
			"parse response failed (original: %s)",
			desc,
		)

	default:
		// Re-wrap other recognized libError values as API_CALL_ERROR
		return libError.NewWithDescription(
			status.InternalServerError,
			"API_CALL_ERROR",
			"API call failed (original: %s): %v",
			desc, err,
		)
	}
}
