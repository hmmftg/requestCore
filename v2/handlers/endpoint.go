package handlers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libRequest"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Endpoint is a non-generic descriptor for a typed v2 route handler.
// It is produced by NewEndpoint[Req, Resp], which captures the request and
// response types inside its constructor closure so that each resource
// operation can use independent types.
//
// An Endpoint carries the handler title, body-binding mode, the typed
// handler function, and optional lifecycle hooks. RegisterEndpoint wires
// it into a routing.RouteGroup with the full BaseHandler lifecycle.
type Endpoint struct {
	Title           string
	Path            string
	Body            libRequest.Type
	ValidateHeader  bool
	LogArrays       []string
	LogTags         []string
	EnableTracing   bool
	TracingSpanName string

	// run executes the typed handler and is set by NewEndpoint. It
	// receives a fully-initialized HandlerRequest and returns the typed
	// response and error. The closure captures the Req/Resp types.
	run func(*endpointRun) (any, error)

	// initializer and finalizer are optional lifecycle hooks captured
	// with the same typed HandlerRequest.
	initializer func(*endpointRun) error
	finalizer   func(*endpointRun)

	// persistence is optional and captured with the same types.
	persistence any // RequestPersister[Req, Resp]

	// recoveryHandler is optional.
	recoveryHandler func(any)

	// idParser, when set, is called before the initializer to parse
	// URL parameters (e.g. resource IDs) and store them in the parser's
	// locals. It receives the v2 RequestContext directly to avoid
	// generic type constraints. Set by resource registration.
	idParser func(ctx *v2wf.RequestContext) error

	// reqType and respType record the reflect.Type of the Req and Resp
	// type parameters passed to NewEndpoint. WithInitializer,
	// WithFinalizer, and WithPersistence check these at registration
	// time to prevent mismatched type hooks from panicking at runtime.
	reqType  reflect.Type
	respType reflect.Type

	// buildCarrier constructs an endpointTrxCarrier bound to the typed
	// HandlerRequest[Req, Resp]. Set by NewEndpoint.
	buildCarrier func(
		core any,
		w legacy.WebFramework,
		ctx *v2wf.RequestContext,
		args []any,
		span trace.Span,
		spanCtx context.Context,
	) *endpointTrxCarrier
}

// endpointRun is the internal carrier passed to Endpoint closures. It
// holds a pointer to the typed HandlerRequest via any to avoid exporting
// generic state on the non-generic Endpoint.
type endpointRun struct {
	trx any // *HandlerRequest[Req, Resp]
}

// NewEndpoint creates a typed Endpoint from a handler function with the
// given title and body-binding mode. The request and response types are
// captured in the returned closure.
func NewEndpoint[Req, Resp any](
	title string,
	body libRequest.Type,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
) *Endpoint {
	e := &Endpoint{
		Title:    title,
		Body:     body,
		reqType:  reflect.TypeOf((*Req)(nil)).Elem(),
		respType: reflect.TypeOf((*Resp)(nil)).Elem(),
		run: func(er *endpointRun) (any, error) {
			trx := er.trx.(*HandlerRequest[Req, Resp])
			return handler(trx.Request, trx)
		},
	}
	e.buildCarrier = func(
		core any,
		w legacy.WebFramework,
		ctx *v2wf.RequestContext,
		args []any,
		span trace.Span,
		spanCtx context.Context,
	) *endpointTrxCarrier {
		var coreIface requestCore.RequestCoreInterface
		if c, ok := core.(requestCore.RequestCoreInterface); ok {
			coreIface = c
		}
		trx := &HandlerRequest[Req, Resp]{
			Title:   title,
			Args:    args,
			Core:    coreIface,
			W:       w,
			V2:      ctx,
			Span:    span,
			SpanCtx: spanCtx,
		}
		carrier := &endpointTrxCarrier{trx: trx}
		carrier.setHeader = func(h *libRequest.RequestHeader) { trx.Header = h }
		carrier.setRequest = func(r any) {
			if r != nil {
				trx.Request = r.(*Req)
			}
		}
		carrier.setResponse = func(r any) {
			if r != nil {
				trx.Response = r.(Resp)
			}
		}
		carrier.setOutcome = func(err error, status int) { trx.SetOutcome(err, status) }
		carrier.setDuration = func(d time.Duration) { trx.Duration = d }
		carrier.markRespSent = func() { trx.RespSent = true }
		carrier.parseRequest = func(wf legacy.WebFramework) (any, *libRequest.RequestHeader, error) {
			req, header, err := libRequest.ParseRequest[Req](wf, body, e.ValidateHeader)
			if err != nil {
				return nil, nil, err
			}
			return req, header, nil // req is *Req
		}
		if e.persistence != nil {
			p := e.persistence.(RequestPersister[Req, Resp])
			carrier.insertPersistence = func(c *endpointTrxCarrier) error {
				return p.Insert(e.Path, c.trx.(*HandlerRequest[Req, Resp]))
			}
			carrier.updatePersistence = func(c *endpointTrxCarrier) error {
				return p.Update(e.Path, c.trx.(*HandlerRequest[Req, Resp]))
			}
		}
		if e.initializer != nil {
			initFn := e.initializer
			carrier.runInitializer = func(c *endpointTrxCarrier) error {
				return initFn(&endpointRun{trx: c.trx})
			}
		}
		carrier.runHandler = func(c *endpointTrxCarrier) (any, error) {
			return e.run(&endpointRun{trx: c.trx})
		}
		if e.finalizer != nil {
			finFn := e.finalizer
			carrier.runFinalizer = func(c *endpointTrxCarrier) {
				finFn(&endpointRun{trx: c.trx})
			}
		}
		return carrier
	}
	return e
}

// WithPath sets the route path for the endpoint.
func (e *Endpoint) WithPath(path string) *Endpoint {
	e.Path = path
	return e
}

// WithHeaderValidation enables request header validation.
func (e *Endpoint) WithHeaderValidation() *Endpoint {
	e.ValidateHeader = true
	return e
}

// WithTracing enables tracing with the given span name.
func (e *Endpoint) WithTracing(spanName string) *Endpoint {
	e.EnableTracing = true
	e.TracingSpanName = spanName
	return e
}

// WithLogArrays registers additional log array tags to collect in the
// finalizer.
func (e *Endpoint) WithLogArrays(tags ...string) *Endpoint {
	e.LogArrays = append(e.LogArrays, tags...)
	return e
}

// WithLogTags registers additional log tag keys to collect in the finalizer.
func (e *Endpoint) WithLogTags(tags ...string) *Endpoint {
	e.LogTags = append(e.LogTags, tags...)
	return e
}

// WithInitializer sets an initializer that runs after parsing and before
// the main handler. This is a free function because Go methods cannot have
// type parameters; the Req/Resp types must match those passed to NewEndpoint.
// A panic occurs at registration time if the types do not match.
func WithInitializer[Req, Resp any](e *Endpoint, fn func(trx *HandlerRequest[Req, Resp]) error) *Endpoint {
	checkEndpointTypes[Req, Resp](e, "WithInitializer")
	e.initializer = func(er *endpointRun) error {
		return fn(er.trx.(*HandlerRequest[Req, Resp]))
	}
	return e
}

// WithFinalizer sets a finalizer that runs after the response is sent.
// This is a free function because Go methods cannot have type parameters.
// A panic occurs at registration time if the types do not match.
func WithFinalizer[Req, Resp any](e *Endpoint, fn func(trx *HandlerRequest[Req, Resp])) *Endpoint {
	checkEndpointTypes[Req, Resp](e, "WithFinalizer")
	e.finalizer = func(er *endpointRun) {
		fn(er.trx.(*HandlerRequest[Req, Resp]))
	}
	return e
}

// WithPersistence sets the request persister for insert/update lifecycle.
// This is a free function because Go methods cannot have type parameters.
// A panic occurs at registration time if the types do not match.
func WithPersistence[Req, Resp any](e *Endpoint, p RequestPersister[Req, Resp]) *Endpoint {
	checkEndpointTypes[Req, Resp](e, "WithPersistence")
	e.persistence = p
	return e
}

// checkEndpointTypes panics if the Req/Resp type parameters of a
// WithInitializer/WithFinalizer/WithPersistence call do not match the
// types captured by NewEndpoint. This provides early, clear feedback
// instead of a runtime type-assertion panic during request processing.
func checkEndpointTypes[Req, Resp any](e *Endpoint, caller string) {
	wantReq := reflect.TypeOf((*Req)(nil)).Elem()
	wantResp := reflect.TypeOf((*Resp)(nil)).Elem()
	if e.reqType != wantReq || e.respType != wantResp {
		panic(fmt.Sprintf("%s: type mismatch: endpoint is %s/%s but hook is %s/%s",
			caller, e.reqType, e.respType, wantReq, wantResp))
	}
}

// WithRecoveryHandler sets a custom panic recovery handler.
func (e *Endpoint) WithRecoveryHandler(fn func(any)) *Endpoint {
	e.recoveryHandler = fn
	return e
}

// WithIDParser sets a URL parameter parser that runs before the initializer.
// It receives the v2 RequestContext directly and can store parsed values
// in the parser's locals for the handler to retrieve via GetParsedID.
func (e *Endpoint) WithIDParser(fn func(ctx *v2wf.RequestContext) error) *Endpoint {
	e.idParser = fn
	return e
}

// RegisterEndpoint registers an Endpoint on the given RouteGroup for the
// specified HTTP method and path. The endpoint's Path is used if the path
// argument is empty.
func RegisterEndpoint(
	router routing.RouteGroup,
	core any, // requestCore.RequestCoreInterface or nil
	respHandler *v2response.Handler,
	method string,
	path string,
	endpoint *Endpoint,
	args ...any,
) error {
	if path == "" {
		path = endpoint.Path
	}
	h := buildEndpointHandler(core, respHandler, endpoint, args)
	return router.Handle(method, path, h)
}

// GetEndpoint registers a GET endpoint.
func GetEndpoint[Req, Resp any](
	router routing.RouteGroup,
	core any,
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
	core any,
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
	core any,
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
	core any,
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
	core any,
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
	core any,
	respHandler *v2response.Handler,
	path string,
	handler func(req *Req, trx *HandlerRequest[Req, Resp]) (Resp, error),
	args ...any,
) error {
	e := NewEndpoint[Req, Resp]("", libRequest.NoBinding, handler).WithPath(path)
	return RegisterEndpoint(router, core, respHandler, "HEAD", path, e, args...)
}
