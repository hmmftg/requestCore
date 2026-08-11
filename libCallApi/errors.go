package libCallApi

import (
	"errors"
	"fmt"
	"net/http"
)

// RemoteCallError preserves the original HTTP status code and raw response body
// from a non-2xx remote API call. It implements error and Unwrap so that callers
// can use errors.As to extract the status code and body for routing decisions.
type RemoteCallError struct {
	Status int    // HTTP status code from the remote response
	Body   []byte // Raw response body for debugging
	Err    error  // Underlying error (e.g. libError.NewWithDescription)
}

// Error returns a human-readable description of the remote call error.
func (e *RemoteCallError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("remote call failed (status %d): %s", e.Status, e.Err.Error())
	}
	return fmt.Sprintf("remote call failed (status %d)", e.Status)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *RemoteCallError) Unwrap() error {
	return e.Err
}

// IsForbidden returns true if err is a RemoteCallError with HTTP status 401 or 403.
func IsForbidden(err error) bool {
	var rce *RemoteCallError
	if errors.As(err, &rce) {
		return rce.Status == http.StatusUnauthorized || rce.Status == http.StatusForbidden
	}
	return false
}

// IsClientError returns true if err is a RemoteCallError with a 4xx status code.
func IsClientError(err error) bool {
	var rce *RemoteCallError
	if errors.As(err, &rce) {
		return rce.Status >= 400 && rce.Status < 500
	}
	return false
}

// IsServerError returns true if err is a RemoteCallError with a 5xx status code.
func IsServerError(err error) bool {
	var rce *RemoteCallError
	if errors.As(err, &rce) {
		return rce.Status >= 500
	}
	return false
}
