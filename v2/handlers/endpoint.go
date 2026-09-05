// Package handlers provides the canonical v2 endpoint descriptors and
// registration helpers built on top of the endpoint.Executor kernel.
//
// The v2 handler signature is canonical:
//
//	func(ctx *request.Context, req Req) (Resp, error)
//
// handlers.Endpoint[Req, Resp] is a thin wrapper around
// endpoint.Endpoint[Req, Resp] that adds operation metadata (operation
// ID, HTTP method, route pattern) and satisfies the non-generic
// EndpointRuntime interface so resources can return heterogeneous typed
// endpoints without exposing type parameters at the registration
// boundary.
//
// Observability flows through telemetry.Sink (configured on the
// endpoint.Executor), not webFramework.AddLog. The v2 kernel is
// stdlib-only and does not import webFramework.
package handlers

import (
	"fmt"

	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/v2/adapter"
	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// Endpoint is a typed descriptor for a v2 route handler. It wraps
// endpoint.Endpoint[Req, Resp] with operation metadata (operation ID,
// HTTP method, route pattern) so it can be registered on a
// routing.RouteGroup via RegisterEndpoint or RegisterRuntime.
//
// The Req and Resp type parameters flow through the entire lifecycle
// without type erasure — bind, validate, execute, encode, and commit
// are all compile-time type-safe inside endpoint.Executor.
//
// An Endpoint is produced by New (or the convenience constructors Get,
// Post, Put, Patch, Delete, Head) and registered on a
// routing.RouteGroup via RegisterEndpoint.
type Endpoint[Req, Resp any] struct {
	inner *endpoint.Endpoint[Req, Resp]
}

// EndpointRuntime is the type-erased interface accepted at the router
// registration boundary. Every *Endpoint[Req, Resp] satisfies it. This
// is the ONLY place type erasure occurs — everything inside the
// endpoint.Executor lifecycle is fully typed.
type EndpointRuntime interface {
	RuntimeHandler(exec *endpoint.Executor) routing.Handler
	OperationID() string
	Method() string
	Pattern() string
}

// ConfigurableEndpoint extends EndpointRuntime with path and ID-parser
// configuration. It is used by resources.Register to set paths and
// inject ID parsers on endpoints returned by Resource[ID] without
// needing to know the concrete Req/Resp types.
type ConfigurableEndpoint interface {
	EndpointRuntime
	SetPath(path string)
	SetIDParser(fn func(ctx *request.Context) error)
}

// New creates a typed Endpoint from a handler function with the given
// operation metadata. The handler signature is the canonical v2
// signature: func(ctx *request.Context, req Req) (Resp, error).
//
// Operation IDs are mandatory in the new API. The method and pattern
// are stored as operation metadata and validated against the
// registration method/path by adapter.Register.
func New[Req, Resp any](
	opID string,
	method string,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	inner := endpoint.New[Req, Resp](
		handler,
		endpoint.WithOperation(operation.Operation{
			ID:      opID,
			Method:  method,
			Pattern: pattern,
		}),
	)
	return &Endpoint[Req, Resp]{inner: inner}
}

// NewWithBinding creates a typed Endpoint with an explicit binding plan.
// This is used by the convenience constructors (Post, Put, Patch) that
// default to JSON binding, and can be used directly for custom binding
// modes (query, path, header, form).
func NewWithBinding[Req, Resp any](
	opID string,
	method string,
	pattern string,
	plan binding.Plan,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	inner := endpoint.New[Req, Resp](
		handler,
		endpoint.WithOperation(operation.Operation{
			ID:      opID,
			Method:  method,
			Pattern: pattern,
		}),
		endpoint.WithBindingPlan(plan),
	)
	return &Endpoint[Req, Resp]{inner: inner}
}

// Inner returns the wrapped endpoint.Endpoint. This provides access to
// the endpoint package's option methods (WithValidator,
// WithSuccessStatus, WithEncoder, etc.) for advanced configuration.
func (e *Endpoint[Req, Resp]) Inner() *endpoint.Endpoint[Req, Resp] {
	return e.inner
}

// OperationID returns the operation's unique identifier.
func (e *Endpoint[Req, Resp]) OperationID() string {
	return e.inner.Config.Operation.ID
}

// Method returns the HTTP method for this endpoint.
func (e *Endpoint[Req, Resp]) Method() string {
	return e.inner.Config.Operation.Method
}

// Pattern returns the route pattern for this endpoint.
func (e *Endpoint[Req, Resp]) Pattern() string {
	return e.inner.Config.Operation.Pattern
}

// SetPath sets the route pattern on the endpoint. Satisfies
// ConfigurableEndpoint for interface-based access from resources.Register.
func (e *Endpoint[Req, Resp]) SetPath(path string) {
	op := e.inner.Config.Operation
	op.Pattern = path
	e.inner.Config.Operation = op
}

// SetIDParser sets the ID parser function on the endpoint. Satisfies
// ConfigurableEndpoint for interface-based access from resources.Register.
// The ID parser runs after binding and before validation.
func (e *Endpoint[Req, Resp]) SetIDParser(fn func(ctx *request.Context) error) {
	e.inner.Config.IDParser = fn
}

// --- Lifecycle configuration methods ---

// WithInitializer sets the initializer hook. The initializer runs
// after validation and before the handler. Returns the same endpoint
// for chaining.
func (e *Endpoint[Req, Resp]) WithInitializer(fn func(ctx *request.Context, req *Req) error) *Endpoint[Req, Resp] {
	e.inner.WithInitializer(fn)
	return e
}

// WithFinalizer sets the finalizer hook. The finalizer runs after the
// response is committed (or after an error). Returns the same endpoint
// for chaining.
func (e *Endpoint[Req, Resp]) WithFinalizer(fn func(ctx *request.Context, req *Req, resp *Resp, err error)) *Endpoint[Req, Resp] {
	e.inner.WithFinalizer(fn)
	return e
}

// WithPersister sets the persister. BeforeExecute runs before the
// handler; AfterCommit runs after the response is committed.
// Returns the same endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithPersister(p endpoint.Persister[Req, Resp]) *Endpoint[Req, Resp] {
	e.inner.WithPersister(p)
	return e
}

// WithTracing enables OpenTelemetry tracing for this endpoint. The
// span name defaults to the operation ID if empty. Returns the same
// endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithTracing(spanName string) *Endpoint[Req, Resp] {
	e.inner.Config.EnableTracing = true
	e.inner.Config.TracingSpanName = spanName
	return e
}

// WithTracer sets a specific OpenTelemetry tracer for this endpoint,
// overriding the executor's default. Returns the same endpoint for
// chaining.
func (e *Endpoint[Req, Resp]) WithTracer(t oteltrace.Tracer) *Endpoint[Req, Resp] {
	e.inner.Config.Tracer = t
	return e
}

// WithRecoveryHandler sets a custom panic recovery handler. Returns
// the same endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithRecoveryHandler(fn func(panicVal any) error) *Endpoint[Req, Resp] {
	e.inner.Config.RecoveryHandler = fn
	return e
}

// RuntimeHandler produces a routing.Handler closure that runs the full
// endpoint.Executor lifecycle for this endpoint. The closure delegates
// to adapter.Wrap, which calls endpoint.Execute with the given executor.
//
// Satisfies EndpointRuntime and ConfigurableEndpoint.
func (e *Endpoint[Req, Resp]) RuntimeHandler(exec *endpoint.Executor) routing.Handler {
	return adapter.Wrap[Req, Resp](e.inner, exec)
}

// Compile-time assertions that *Endpoint satisfies the interfaces.
var (
	_ EndpointRuntime      = (*Endpoint[struct{}, struct{}])(nil)
	_ ConfigurableEndpoint = (*Endpoint[struct{}, struct{}])(nil)
)

// RegisterEndpoint registers a typed Endpoint on the given RouteGroup
// using the endpoint's own method and pattern. The executor runs the
// full lifecycle (bind → validate → execute → encode → commit →
// observe) and provides telemetry via its configured telemetry.Sink.
func RegisterEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	ep *Endpoint[Req, Resp],
) error {
	if ep == nil {
		return fmt.Errorf("handlers: nil endpoint")
	}
	return adapter.Register[Req, Resp](router, exec, ep.Method(), ep.Pattern(), ep.inner)
}

// RegisterRuntime registers any EndpointRuntime on the given RouteGroup.
// This is used by resources.Register where the concrete Req/Resp types
// are not known (the resource returns EndpointRuntime).
//
// The endpoint's own Method and Pattern are used for registration.
func RegisterRuntime(
	router routing.RouteGroup,
	exec *endpoint.Executor,
	ep EndpointRuntime,
) error {
	if ep == nil {
		return fmt.Errorf("handlers: nil endpoint runtime")
	}
	return router.Handle(ep.Method(), ep.Pattern(), ep.RuntimeHandler(exec))
}

// --- Convenience constructors ---

// Get creates a GET endpoint with no body binding.
func Get[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return New[Req, Resp](opID, "GET", pattern, handler)
}

// Post creates a POST endpoint with JSON body binding.
func Post[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return NewWithBinding[Req, Resp](opID, "POST", pattern, binding.DefaultJSONPlan, handler)
}

// Put creates a PUT endpoint with JSON body binding.
func Put[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return NewWithBinding[Req, Resp](opID, "PUT", pattern, binding.DefaultJSONPlan, handler)
}

// Patch creates a PATCH endpoint with JSON body binding.
func Patch[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return NewWithBinding[Req, Resp](opID, "PATCH", pattern, binding.DefaultJSONPlan, handler)
}

// Delete creates a DELETE endpoint with no body binding.
func Delete[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return New[Req, Resp](opID, "DELETE", pattern, handler)
}

// Head creates a HEAD endpoint with no body binding.
func Head[Req, Resp any](
	opID, pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) *Endpoint[Req, Resp] {
	return New[Req, Resp](opID, "HEAD", pattern, handler)
}

// --- Legacy convenience constructors (register + create in one call) ---

// GetEndpoint creates and registers a GET endpoint in one call.
func GetEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Get[Req, Resp](defaultOpID("GET", pattern), pattern, handler))
}

// PostEndpoint creates and registers a POST endpoint with JSON binding.
func PostEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Post[Req, Resp](defaultOpID("POST", pattern), pattern, handler))
}

// PutEndpoint creates and registers a PUT endpoint with JSON binding.
func PutEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Put[Req, Resp](defaultOpID("PUT", pattern), pattern, handler))
}

// PatchEndpoint creates and registers a PATCH endpoint with JSON binding.
func PatchEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Patch[Req, Resp](defaultOpID("PATCH", pattern), pattern, handler))
}

// DeleteEndpoint creates and registers a DELETE endpoint.
func DeleteEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Delete[Req, Resp](defaultOpID("DELETE", pattern), pattern, handler))
}

// HeadEndpoint creates and registers a HEAD endpoint.
func HeadEndpoint[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	pattern string,
	handler func(ctx *request.Context, req Req) (Resp, error),
) error {
	return RegisterEndpoint(router, exec, Head[Req, Resp](defaultOpID("HEAD", pattern), pattern, handler))
}

// defaultOpID generates a deterministic operation ID from the method
// and pattern when the caller does not provide one via the standalone
// constructors. The pattern is sanitized: leading/trailing slashes are
// stripped and internal slashes become hyphens.
func defaultOpID(method, pattern string) string {
	s := pattern
	// Strip leading slash.
	for len(s) > 0 && s[0] == '/' {
		s = s[1:]
	}
	// Strip trailing slash.
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	// Replace slashes and path params with hyphens.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '/':
			out = append(out, '-')
		case '{', '}':
			// skip param braces
		default:
			out = append(out, c)
		}
	}
	return method + ":" + string(out)
}
