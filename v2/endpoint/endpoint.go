// package endpoint provides the internal typed endpoint descriptor and
// lifecycle executor for the v2 kernel.
//
// This package is internal: it cannot be imported outside v2/. Tranche
// 4 promotes tested types to canonical public paths.
//
// The canonical handler signature is:
//
//	func(*request.Context, Req) (Resp, error)
//
// The Executor runs the full lifecycle:
// bind → validate → execute → encode → prepare → hooks → durable →
// commit → observe.
//
// endpoint imports request, operation, response, binding, validation,
// telemetry, and request/faketransport. It does not import handlers,
// app, routing, adapters, or any v1 package.
package endpoint

import (
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/validation"
)

// Encoder serializes a response value into bytes and declares its
// content type. It is structurally compatible with renderers.Renderer
// so that JSONRenderer, XMLRenderer, TextRenderer, CSVRenderer, and
// custom renderers can be used as endpoint encoders without wrappers.
type Encoder interface {
	Encode(data any) ([]byte, error)
	ContentType() string
}

// Config holds the non-generic metadata for an Endpoint. Options
// operate on Config, avoiding the need for generic option functions.
type Config struct {
	// Operation is the declarative operation metadata.
	Operation operation.Operation

	// BindingPlan describes how to decode the request into Req.
	// If Mode is ModeNone, binding is skipped.
	BindingPlan binding.Plan

	// Validator validates the bound Req. If nil, validation is
	// skipped.
	Validator *validation.Validator

	// SuccessStatus is the HTTP status code for successful responses.
	// Defaults to 200 if zero.
	SuccessStatus int

	// SuccessContentType is the Content-Type for successful responses.
	// Defaults to "application/json" if empty. If an Encoder is set,
	// its ContentType takes precedence unless the handler explicitly
	// sets a Content-Type header.
	SuccessContentType string

	// Encoder overrides the executor's default encoder for this
	// endpoint. If nil, the executor's default encoder (or JSON) is
	// used. An endpoint that needs XML, text, CSV, or custom encoding
	// sets this to the appropriate renderers.Renderer.
	Encoder Encoder

	// DeclaredProblems lists the RFC 9457 problem types this endpoint
	// may produce. Used for OpenAPI generation and documentation.
	DeclaredProblems []response.Problem

	// Tags categorize the endpoint for documentation and routing.
	Tags []string

	// Deprecated marks the endpoint as deprecated.
	Deprecated bool

	// Tracer is the OpenTelemetry tracer for this endpoint. If nil,
	// the executor's tracer is used. If both are nil, tracing is
	// disabled (a no-op tracer is used).
	Tracer oteltrace.Tracer

	// EnableTracing controls whether a span is created for this
	// endpoint. If false, no span is created even if a tracer is
	// configured. Default: false.
	EnableTracing bool

	// TracingSpanName is the name for the OpenTelemetry span. If
	// empty, the operation ID is used.
	TracingSpanName string

	// RecoveryHandler is called when the handler panics. If set, it
	// receives the panic value and may return a domain-specific error
	// that the ProblemMapper can map to an appropriate RFC 9457
	// Problem. If it returns nil, a generic "handler panic" error is
	// used. If nil, the default behavior (generic panic error) is
	// preserved.
	RecoveryHandler func(panicVal any) error

	// IDParser runs after binding and before validation. It can
	// validate and parse path parameters, storing the result on the
	// request context. If it returns an error, a 400 Problem is
	// written.
	IDParser func(ctx *request.Context) error
}

// Option configures a Config at construction time. Options are
// non-generic so they can be inferred without explicit type
// parameters.
type Option func(*Config)

// Endpoint captures a typed handler and its declarative metadata. It
// is generic over the request and response types so the executor can
// bind and encode without type assertions.
type Endpoint[Req, Resp any] struct {
	// Handler is the canonical handler function.
	Handler func(*request.Context, Req) (Resp, error)

	// Config holds the non-generic metadata.
	Config Config

	// Initializer runs after validation and before the handler. If it
	// returns an error, the request is aborted with a mapped Problem.
	Initializer func(ctx *request.Context, req *Req) error

	// Finalizer runs after the response is committed (or after an
	// error), in a defer so it always runs. It receives the response
	// pointer and error. On the success path, resp is the handler's
	// response and err is nil. On the error path, resp is the zero
	// value and err is the error. Finalizer errors are logged via
	// telemetry but not propagated.
	Finalizer func(ctx *request.Context, req *Req, resp *Resp, err error)

	// Persister optionally persists request lifecycle data.
	// BeforeExecute runs after the initializer and before the handler;
	// failure aborts the request. AfterCommit runs after the response
	// is committed; failures are best-effort (logged, not propagated).
	Persister Persister[Req, Resp]
}

// New creates a new Endpoint with the given handler and options.
func New[Req, Resp any](handler func(*request.Context, Req) (Resp, error), opts ...Option) *Endpoint[Req, Resp] {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Endpoint[Req, Resp]{
		Handler: handler,
		Config:  cfg,
	}
}

// WithOperation sets the operation metadata.
func WithOperation(op operation.Operation) Option {
	return func(c *Config) { c.Operation = op }
}

// WithBindingPlan sets the binding plan.
func WithBindingPlan(plan binding.Plan) Option {
	return func(c *Config) { c.BindingPlan = plan }
}

// WithValidator sets the validator.
func WithValidator(v *validation.Validator) Option {
	return func(c *Config) { c.Validator = v }
}

// WithSuccessStatus sets the success HTTP status code.
func WithSuccessStatus(status int) Option {
	return func(c *Config) { c.SuccessStatus = status }
}

// WithSuccessContentType sets the success Content-Type.
func WithSuccessContentType(ct string) Option {
	return func(c *Config) { c.SuccessContentType = ct }
}

// WithEncoder sets the response encoder for this endpoint, overriding
// the executor's default. The encoder's ContentType is used unless the
// handler explicitly sets a Content-Type header.
func WithEncoder(enc Encoder) Option {
	return func(c *Config) { c.Encoder = enc }
}

// WithDeclaredProblems sets the declared problem types.
func WithDeclaredProblems(problems []response.Problem) Option {
	return func(c *Config) { c.DeclaredProblems = problems }
}

// WithTags sets the endpoint tags.
func WithTags(tags []string) Option {
	return func(c *Config) { c.Tags = tags }
}

// WithDeprecated marks the endpoint as deprecated.
func WithDeprecated() Option {
	return func(c *Config) { c.Deprecated = true }
}

// WithTracer sets the OpenTelemetry tracer for this endpoint.
func WithTracer(t oteltrace.Tracer) Option {
	return func(c *Config) { c.Tracer = t }
}

// WithTracing enables tracing and sets the span name. If spanName is
// empty, the operation ID is used.
func WithTracing(spanName string) Option {
	return func(c *Config) {
		c.EnableTracing = true
		c.TracingSpanName = spanName
	}
}

// WithRecoveryHandler sets a custom panic recovery handler.
func WithRecoveryHandler(fn func(panicVal any) error) Option {
	return func(c *Config) { c.RecoveryHandler = fn }
}

// WithIDParser sets a pre-handler ID parser that runs after binding.
func WithIDParser(fn func(ctx *request.Context) error) Option {
	return func(c *Config) { c.IDParser = fn }
}

// --- Generic lifecycle methods on *Endpoint[Req, Resp] ---

// WithInitializer sets the initializer hook on the endpoint. The
// initializer runs after validation and before the handler. Returns
// the same endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithInitializer(fn func(ctx *request.Context, req *Req) error) *Endpoint[Req, Resp] {
	e.Initializer = fn
	return e
}

// WithFinalizer sets the finalizer hook on the endpoint. The finalizer
// runs after the response is committed (or after an error). Returns
// the same endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithFinalizer(fn func(ctx *request.Context, req *Req, resp *Resp, err error)) *Endpoint[Req, Resp] {
	e.Finalizer = fn
	return e
}

// WithPersister sets the persister on the endpoint. Returns the same
// endpoint for chaining.
func (e *Endpoint[Req, Resp]) WithPersister(p Persister[Req, Resp]) *Endpoint[Req, Resp] {
	e.Persister = p
	return e
}

// --- Persister interface ---

// Persister optionally persists request lifecycle data. The executor
// calls BeforeExecute after the initializer and before the handler;
// failure aborts the request. AfterCommit runs after the response is
// committed; failures are best-effort (logged via telemetry, not
// propagated).
//
// Implementations may implement only one phase by returning nil from
// the other.
type Persister[Req, Resp any] interface {
	// BeforeExecute runs before the handler. If it returns an error,
	// the request is aborted with a mapped Problem.
	BeforeExecute(ctx *request.Context, req *Req) error

	// AfterCommit runs after the response is committed. It receives
	// the request, response, and error (non-nil on error paths).
	// Failures are best-effort and logged via telemetry.
	AfterCommit(ctx *request.Context, req *Req, resp *Resp, err error) error
}

// FuncPersister is a Persister backed by function fields. Either
// function may be nil, in which case that phase is skipped.
type FuncPersister[Req, Resp any] struct {
	BeforeExecuteFn func(ctx *request.Context, req *Req) error
	AfterCommitFn   func(ctx *request.Context, req *Req, resp *Resp, err error) error
}

// BeforeExecute delegates to BeforeExecuteFn if set.
func (p *FuncPersister[Req, Resp]) BeforeExecute(ctx *request.Context, req *Req) error {
	if p.BeforeExecuteFn == nil {
		return nil
	}
	return p.BeforeExecuteFn(ctx, req)
}

// AfterCommit delegates to AfterCommitFn if set.
func (p *FuncPersister[Req, Resp]) AfterCommit(ctx *request.Context, req *Req, resp *Resp, err error) error {
	if p.AfterCommitFn == nil {
		return nil
	}
	return p.AfterCommitFn(ctx, req, resp, err)
}
