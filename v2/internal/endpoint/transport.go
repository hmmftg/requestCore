package endpoint

import (
	"net/http"

	"github.com/hmmftg/requestCore/v2/request/faketransport"
)

// Transport abstracts the response-writing side of an HTTP request.
// The executor uses it to write the encoded response body and check
// commit state.
type Transport interface {
	// WriteResponse writes the HTTP response with the given status,
	// content type, and body. It is called exactly once per request.
	WriteResponse(status int, contentType string, body []byte) error

	// Committed reports whether the response has already been
	// committed to the wire. Once true, WriteResponse must not be
	// called again.
	Committed() bool
}

// FakeTransportAdapter adapts a faketransport.FakeTransport to the
// Transport interface. It is used by tests and benchmarks to exercise
// the executor without a real HTTP server.
type FakeTransportAdapter struct {
	FT *faketransport.FakeTransport
}

// WriteResponse writes the response to the underlying FakeTransport.
func (a *FakeTransportAdapter) WriteResponse(status int, contentType string, body []byte) error {
	if a.FT.Committed() {
		return nil
	}
	rec := a.FT.Recorder()
	if contentType != "" {
		rec.Header().Set("Content-Type", contentType)
	}
	rec.WriteHeader(status)
	_, err := rec.Write(body)
	a.FT.MarkCommitted()
	return err
}

// Committed reports whether the response has been committed.
func (a *FakeTransportAdapter) Committed() bool {
	return a.FT.Committed()
}

// Compile-time interface checks.
var (
	_ Transport = (*FakeTransportAdapter)(nil)
)

// httpStatus returns the HTTP status code for an error if it
// implements HTTPStatus() int, otherwise 0.
func httpStatusOf(err error) int {
	type statusProvider interface {
		HTTPStatus() int
	}
	if sp, ok := err.(statusProvider); ok {
		return sp.HTTPStatus()
	}
	return 0
}

// ensure net/http is used (for status constants referenced in executor).
var _ = http.StatusOK
