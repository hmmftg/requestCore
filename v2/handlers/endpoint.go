package handlers

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/response"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Endpoint is a typed descriptor for a v2 route handler. The Req and
// Resp type parameters flow through the entire lifecycle without type
// erasure — parse, initialize, handle, render, and finalize are all
// compile-time type-safe.
//
// An Endpoint is produced by NewEndpoint[Req, Resp] and registered on a
// routing.RouteGroup via RegisterEndpoint. Optional lifecycle hooks
// (initializer, finalizer, persistence) are set via the With* methods,
// which are fully typed and checked at compile time.
type Endpoint[Req, Resp any] struct {
	Title           string
	Path            string
	Body            libRequest.Type
	ValidateHeader  bool
	LogArrays       []string
	LogTags         []string
	EnableTracing   bool
	TracingSpanName string

	handler         func(*Req, *HandlerRequest[Req, Resp]) (Resp, error)
	initializer     func(*HandlerRequest[Req, Resp]) error
	finalizer       func(*HandlerRequest[Req, Resp])
	persistence     RequestPersister[Req, Resp]
	recoveryHandler func(any)
	idParser        func(*v2wf.RequestContext) error
	args            []any
}

// EndpointRuntime is the type-erased interface accepted at the router
// registration boundary. Every *Endpoint[Req, Resp] satisfies it via
// RuntimeHandler. This is the ONLY place type erasure occurs —
// everything inside the lifecycle is fully typed.
type EndpointRuntime interface {
	RuntimeHandler(core requestCore.RequestCoreInterface, respHandler *v2response.Handler) routing.Handler
}

// ConfigurableEndpoint extends EndpointRuntime with path and ID-parser
// configuration. It is used by resources.Register to set paths and
// inject ID parsers on endpoints returned by Resource[ID] without
// needing to know the concrete Req/Resp types.
type ConfigurableEndpoint interface {
	EndpointRuntime
	SetPath(path string)
	SetIDParser(fn func(*v2wf.RequestContext) error)
}

// NewEndpoint creates a typed Endpoint from a handler function with the
// given title and body-binding mode. The request and response types are
// captured as type parameters on the returned *Endpoint[Req, Resp].
func NewEndpoint[Req, Resp any](
	title string,
	body libRequest.Type,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
) *Endpoint[Req, Resp] {
	return &Endpoint[Req, Resp]{
		Title:   title,
		Body:    body,
		handler: handler,
	}
}

// WithPath sets the route path. Returns the same typed Endpoint for
// chaining.
func (e *Endpoint[Req, Resp]) WithPath(path string) *Endpoint[Req, Resp] {
	e.Path = path
	return e
}

// SetPath sets the route path without returning the endpoint. Satisfies
// ConfigurableEndpoint for interface-based access from resources.Register.
func (e *Endpoint[Req, Resp]) SetPath(path string) {
	e.Path = path
}

// WithHeaderValidation enables request header validation.
func (e *Endpoint[Req, Resp]) WithHeaderValidation() *Endpoint[Req, Resp] {
	e.ValidateHeader = true
	return e
}

// WithTracing enables tracing with the given span name.
func (e *Endpoint[Req, Resp]) WithTracing(spanName string) *Endpoint[Req, Resp] {
	e.EnableTracing = true
	e.TracingSpanName = spanName
	return e
}

// WithLogArrays registers additional log array tags to collect in the
// finalizer.
func (e *Endpoint[Req, Resp]) WithLogArrays(tags ...string) *Endpoint[Req, Resp] {
	e.LogArrays = append(e.LogArrays, tags...)
	return e
}

// WithLogTags registers additional log tag keys to collect in the finalizer.
func (e *Endpoint[Req, Resp]) WithLogTags(tags ...string) *Endpoint[Req, Resp] {
	e.LogTags = append(e.LogTags, tags...)
	return e
}

// WithInitializer sets an initializer that runs after parsing and before
// the main handler. Fully typed — compile-time checked, no reflection.
func (e *Endpoint[Req, Resp]) WithInitializer(fn func(trx *HandlerRequest[Req, Resp]) error) *Endpoint[Req, Resp] {
	e.initializer = fn
	return e
}

// WithFinalizer sets a finalizer that runs after the response is sent.
// Fully typed — compile-time checked, no reflection.
func (e *Endpoint[Req, Resp]) WithFinalizer(fn func(trx *HandlerRequest[Req, Resp])) *Endpoint[Req, Resp] {
	e.finalizer = fn
	return e
}

// WithPersistence sets the request persister for insert/update lifecycle.
// Fully typed — compile-time checked, no reflection.
func (e *Endpoint[Req, Resp]) WithPersistence(p RequestPersister[Req, Resp]) *Endpoint[Req, Resp] {
	e.persistence = p
	return e
}

// WithRecoveryHandler sets a custom panic recovery handler.
func (e *Endpoint[Req, Resp]) WithRecoveryHandler(fn func(any)) *Endpoint[Req, Resp] {
	e.recoveryHandler = fn
	return e
}

// WithIDParser sets a URL parameter parser that runs before the initializer.
// It receives the v2 RequestContext directly and can store parsed values
// in the parser's locals for the handler to retrieve via GetParsedID.
func (e *Endpoint[Req, Resp]) WithIDParser(fn func(ctx *v2wf.RequestContext) error) *Endpoint[Req, Resp] {
	e.idParser = fn
	return e
}

// SetIDParser sets the ID parser without returning the endpoint. Satisfies
// ConfigurableEndpoint for interface-based access from resources.Register.
func (e *Endpoint[Req, Resp]) SetIDParser(fn func(*v2wf.RequestContext) error) {
	e.idParser = fn
}

// RuntimeHandler produces a routing.Handler closure that runs the full
// BaseHandler lifecycle for this endpoint. The closure captures the
// endpoint's typed Req/Resp and constructs a typed HandlerRequest
// directly — no type erasure, no carrier struct, no reflection.
//
// Satisfies EndpointRuntime and ConfigurableEndpoint.
func (e *Endpoint[Req, Resp]) RuntimeHandler(
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
) routing.Handler {
	if respHandler == nil {
		// Install a handler with a default fallback so the registry
		// never encounters a fallback-less state. This prevents
		// silent failures when respHandler is not explicitly configured.
		reg := v2response.NewRegistry(nil)
		reg.SetFallback(v2response.LegacyFallback(response.WebHanlder{}))
		respHandler = v2response.NewHandler(reg, nil, response.WebHanlder{})
	}
	return func(ctx *v2wf.RequestContext) (err error) {
		start := time.Now()

		// Initialize the legacy WebFramework from the v2 RequestContext's
		// LegacyContext. This preserves the v1 parser/tracing pipeline.
		var w legacy.WebFramework
		if ctx.LegacyContext != nil {
			if e.persistence != nil {
				w = libContext.InitContext(ctx.LegacyContext)
			} else {
				w = libContext.InitContextNoAuditTrail(ctx.LegacyContext)
			}
		} else {
			// Fall back to the v2-carried legacy framework (tests).
			w = ctx.Legacy
		}
		// Ensure the v2 parser and legacy parser are the same instance.
		if ctx.Legacy.Parser == nil && w.Parser != nil {
			ctx.Legacy.Parser = w.Parser
		}
		// AddWebLogs adds request metadata (title/method/path) as log tags
		// and returns a completion closure that logs elapsed time and
		// status and collects the HandlerLogTag tags/arrays. Capture it
		// and invoke exactly once in finalize so status is recorded
		// alongside elapsed in the mandatory handler log collection.
		logCompletion := libContext.AddWebLogs(w, e.Title, legacy.HandlerLogTag)

		// Initialize tracing if enabled.
		var span trace.Span
		var spanCtx context.Context
		if e.EnableTracing {
			spanName := e.TracingSpanName
			if spanName == "" {
				spanName = e.Title
			}
			tm := libTracing.GetGlobalTracingManager()
			if tm != nil {
				spanCtx, span = tm.StartSpanWithAttributes(w.Ctx, spanName, map[string]string{
					"handler.title": e.Title,
					"handler.path":  e.Path,
				})
			}
			if span != nil && span.IsRecording() {
				span.SetAttributes(
					attribute.String("handler.title", e.Title),
					attribute.String("handler.path", e.Path),
					attribute.Bool("handler.validate_header", e.ValidateHeader),
					attribute.Bool("handler.has_persistence", e.persistence != nil),
				)
			}
			defer func() {
				if span != nil {
					if err != nil {
						span.RecordError(err)
						span.SetAttributes(attribute.Int("handler.error_status", committedStatus(ctx)))
					}
					span.SetAttributes(attribute.Int("handler.status", committedStatus(ctx)))
					span.End()
				}
			}()
		}

		// Build the typed HandlerRequest directly — no carrier needed.
		trx := &HandlerRequest[Req, Resp]{
			Title:   e.Title,
			Core:    core,
			Args:    e.args,
			W:       w,
			V2:      ctx,
			Span:    span,
			SpanCtx: spanCtx,
		}

		// Panic recovery + finalization + log collection.
		var requestInserted bool
		var panicVal any
		func() {
			defer func() {
				panicVal = recover()
			}()
			err = runLifecycle(e, respHandler, w, ctx, trx, &requestInserted, start)
		}()

		finalize(e, respHandler, w, ctx, trx, start, requestInserted, panicVal, logCompletion)

		// If a panic occurred, it was converted to a sanitized error
		// response in finalize. Return nil so the router does not
		// double-write.
		if panicVal != nil {
			return nil
		}
		if err != nil && !ctx.Committed() {
			return err
		}
		return nil
	}
}

// RegisterEndpoint registers a typed Endpoint on the given RouteGroup
// for the specified HTTP method and path. The endpoint's Path is used
// if the path argument is empty.
func RegisterEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	method string,
	path string,
	endpoint *Endpoint[Req, Resp],
	args ...any,
) error {
	if path == "" {
		path = endpoint.Path
	}
	if len(args) > 0 {
		endpoint.args = args
	}
	return router.Handle(method, path, endpoint.RuntimeHandler(core, respHandler))
}

// RegisterRuntime registers any EndpointRuntime on the given RouteGroup.
// This is used by resources.Register where the concrete Req/Resp types
// are not known (the resource returns EndpointRuntime).
func RegisterRuntime(
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	method string,
	path string,
	ep EndpointRuntime,
) error {
	return router.Handle(method, path, ep.RuntimeHandler(core, respHandler))
}

// GetEndpoint registers a GET endpoint.
func GetEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.NoBinding, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "GET", path, e, args...)
}

// PostEndpoint registers a POST endpoint with JSON body binding.
func PostEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.JSON, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "POST", path, e, args...)
}

// PutEndpoint registers a PUT endpoint with JSON body binding.
func PutEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.JSON, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "PUT", path, e, args...)
}

// PatchEndpoint registers a PATCH endpoint with JSON body binding.
func PatchEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.JSON, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "PATCH", path, e, args...)
}

// DeleteEndpoint registers a DELETE endpoint.
func DeleteEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.NoBinding, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "DELETE", path, e, args...)
}

// HeadEndpoint registers a HEAD endpoint.
func HeadEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core requestCore.RequestCoreInterface,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.NoBinding, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "HEAD", path, e, args...)
}
