// Package status provides HTTP and application status code constants.
package status

import (
	"fmt"
	"net/http"
)

// StatusCode represents an HTTP or application-level status code.
type StatusCode int

const (
	// OK is the status code for successful requests (HTTP 200).
	OK StatusCode = http.StatusOK
	// Unknown is the status code for unknown errors (HTTP 500).
	Unknown StatusCode = http.StatusInternalServerError
	// InternalServerError is the status code for internal server errors (HTTP 500).
	InternalServerError StatusCode = http.StatusInternalServerError
	// BadRequest is the status code for bad request errors (HTTP 400).
	BadRequest StatusCode = http.StatusBadRequest
	// DuplicateRequest is the status code for duplicate/too many requests (HTTP 429).
	DuplicateRequest StatusCode = http.StatusTooManyRequests
	// NotFound is the status code for not found errors (HTTP 404).
	NotFound StatusCode = http.StatusNotFound
)

// String returns a formatted string representation of the status code (e.g. "200-OK").
func (s StatusCode) String() string {
	return fmt.Sprintf("%d-%s", s, http.StatusText(int(s)))
}

// Int returns the status code as a plain integer.
func (s StatusCode) Int() int {
	return int(s)
}
