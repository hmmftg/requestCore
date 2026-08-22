package webFramework

// ErrorContext holds the context for an error handler invocation.
type ErrorContext struct {
	// Request is the v2 request context.
	Request *RequestContext

	// Error is the error to handle.
	Error error

	// Status is the resolved HTTP status code for this error.
	Status int
}

// ErrorHandler processes an error and writes a response.
// It must call Request.Parser.SendResponse (or the legacy responder)
// to write the error response. If the handler fails to write a response,
// the registry will invoke the fallback handler exactly once.
type ErrorHandler func(ErrorContext) error

// StatusResolver determines the HTTP status code for an error.
// Implementations should use errors.As to inspect wrapped errors
// and extract status from libError.Error, response.ErrorState, or
// any type implementing interface{ HTTPStatus() int }.
type StatusResolver func(error) int
