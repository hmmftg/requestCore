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
