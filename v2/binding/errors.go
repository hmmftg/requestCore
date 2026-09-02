package binding

import (
	"errors"
	"net/http"
)

// Sentinel errors for binding failures. These are wrapped in
// BindingError by the Bind function to carry HTTP status codes.
var (
	// ErrBodyTooLarge indicates the request body exceeded MaxBodyBytes.
	// Maps to HTTP 413.
	ErrBodyTooLarge = errors.New("binding: body too large")

	// ErrTrailingData indicates trailing data after a JSON value.
	// Maps to HTTP 400.
	ErrTrailingData = errors.New("binding: trailing data after JSON value")

	// ErrInvalidJSON indicates invalid JSON in the request body.
	// Maps to HTTP 400.
	ErrInvalidJSON = errors.New("binding: invalid JSON")

	// ErrInvalidContentType indicates an unsupported content type.
	// Maps to HTTP 415.
	ErrInvalidContentType = errors.New("binding: invalid content type")
)

// BindingError wraps a binding failure with an HTTP status code and
// optional field name. It implements the error interface and provides
// HTTPStatus() int for integration with the response problem mapper.
type BindingError struct {
	// Status is the HTTP status code for this error.
	Status int

	// Cause is the sentinel error (ErrBodyTooLarge, ErrTrailingData, etc.).
	Cause error

	// Field is the field name that caused the error, if applicable.
	Field string

	// Message is a human-readable description of the error.
	Message string
}

// Error implements the error interface.
func (e *BindingError) Error() string {
	if e.Field != "" {
		return e.Cause.Error() + ": " + e.Field + ": " + e.Message
	}
	return e.Cause.Error() + ": " + e.Message
}

// Unwrap returns the underlying cause error.
func (e *BindingError) Unwrap() error {
	return e.Cause
}

// HTTPStatus returns the HTTP status code for this binding error.
func (e *BindingError) HTTPStatus() int {
	if e.Status == 0 {
		return http.StatusBadRequest
	}
	return e.Status
}

// Is reports whether this error matches the target sentinel error.
func (e *BindingError) Is(target error) bool {
	return errors.Is(e.Cause, target)
}
