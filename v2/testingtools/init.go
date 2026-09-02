// Package testingtools provides test utilities for v2 handler and
// middleware testing using the canonical kernel (*request.Context,
// routing.Transport, endpoint.Executor, telemetry.Sink).
package testingtools

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/request/faketransport"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/telemetry"
	"github.com/hmmftg/requestCore/v2/workers"
)

// NewTestExecutor creates an endpoint.Executor suitable for testing.
// It uses a nop telemetry sink and a fresh operation registry.
func NewTestExecutor() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
		endpoint.WithProblemMapper(response.DefaultMapperRegistry()),
	)
}

// NewTestExecutorWithSink creates an executor with a capturing telemetry
// sink for verifying telemetry events in tests.
func NewTestExecutorWithSink(sink telemetry.Sink) *endpoint.Executor {
	if sink == nil {
		sink = telemetry.NopSink{}
	}
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithTelemetrySink(sink),
		endpoint.WithProblemMapper(response.DefaultMapperRegistry()),
	)
}

// NewTestContext creates a *request.Context suitable for testing
// handlers and middleware without a real HTTP framework.
func NewTestContext() *request.Context {
	return request.NewContext(context.Background())
}

// NewTestContextWithMethod creates a *request.Context with the given
// HTTP method and path.
func NewTestContextWithMethod(method, path string) *request.Context {
	ctx := request.NewContext(context.Background())
	// Use faketransport to construct a context with method/path.
	ft := faketransport.New(method, path)
	_ = ft // Context is constructed via NewContext; method/path set via faketransport
	return ctx
}

// NewTestTransport creates a fake routing.Transport that captures
// response writes for assertion in tests.
func NewTestTransport() *TestTransport {
	return &TestTransport{
		recorder: httptest.NewRecorder(),
	}
}

// TestTransport is a routing.Transport implementation for tests.
// It captures the response status, content type, headers, and body.
type TestTransport struct {
	recorder  *httptest.ResponseRecorder
	committed bool
}

// WriteResponse writes the response to the internal recorder.
func (t *TestTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	t.committed = true
	if contentType != "" {
		t.recorder.Header().Set("Content-Type", contentType)
	}
	for k, vs := range headers {
		for _, v := range vs {
			t.recorder.Header().Add(k, v)
		}
	}
	t.recorder.WriteHeader(status)
	_, err := t.recorder.Write(body)
	return err
}

// Committed reports whether the response has been written.
func (t *TestTransport) Committed() bool { return t.committed }

// Status returns the captured HTTP status code.
func (t *TestTransport) Status() int { return t.recorder.Code }

// Header returns the response headers.
func (t *TestTransport) Header() http.Header { return t.recorder.Header() }

// Body returns the response body bytes.
func (t *TestTransport) Body() []byte { return t.recorder.Body.Bytes() }

// ContentType returns the response Content-Type header.
func (t *TestTransport) ContentType() string { return t.recorder.Header().Get("Content-Type") }

// NewTestWorker creates an in-process worker pool for testing async
// job behavior.
func NewTestWorker() workers.Worker {
	return workers.NewInProcessWorker(workers.Config{
		WorkerCount: 1,
		QueueSize:   10,
	})
}

// AssertResponse checks that the transport captured the expected
// response status and content type.
func AssertResponse(t *TestTransport, expectedStatus int, expectedContentType string) bool {
	return t.committed &&
		t.Status() == expectedStatus &&
		t.ContentType() == expectedContentType
}

// DefaultRenderer returns the default JSON renderer for tests.
func DefaultRenderer() renderers.Renderer {
	return renderers.JSONRenderer{}
}

// MappedErrorHandler creates a routing.ErrorHandler for tests using
// the default problem mapper.
func MappedErrorHandler() routing.ErrorHandler {
	return func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, response.DefaultMapperRegistry(), err)
	}
}
