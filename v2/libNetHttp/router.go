package libNetHttp

import (
	"context"
	"net/http"

	legacyLibNetHttp "github.com/hmmftg/requestCore/libNetHttp"
	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// NetHTTPRouter implements routing.Router for net/http using http.ServeMux.
// It is the base router for the net/http adapter. The chi adapter wraps
// this to provide richer routing features.
type NetHTTPRouter struct {
	mux                   *http.ServeMux
	middlewares           []routing.Middleware
	notFound              routing.Handler
	methodNA              routing.Handler
	responseWriterFactory func(*http.Request, http.ResponseWriter) *NetHTTPParserV2
	registry              response.Registry
	respHandler           *response.Handler
}

// NewRouter creates a new NetHTTPRouter with a new http.ServeMux.
func NewRouter() *NetHTTPRouter {
	return &NetHTTPRouter{
		mux:                   http.NewServeMux(),
		responseWriterFactory: InitContextV2,
	}
}

// NewRouterFromMux creates a new NetHTTPRouter from an existing http.ServeMux.
func NewRouterFromMux(mux *http.ServeMux) *NetHTTPRouter {
	return &NetHTTPRouter{
		mux:                   mux,
		responseWriterFactory: InitContextV2,
	}
}

// SetErrorHandler installs the v2 response handler and error registry used
// for centralized error dispatch.
func (r *NetHTTPRouter) SetErrorHandler(handler *response.Handler) {
	if handler == nil {
		return
	}
	r.respHandler = handler
	r.registry = handler.Registry()
}

// Native returns the underlying http.ServeMux.
func (r *NetHTTPRouter) Native() any {
	return r.mux
}

// Group creates a sub-group with the given prefix.
// net/http's ServeMux doesn't have native groups, so we prefix patterns.
func (r *NetHTTPRouter) Group(prefix string) routing.RouteGroup {
	return &netHTTPSubGroup{
		parent:      r,
		prefix:      prefix,
		middlewares: r.middlewares,
	}
}

// With returns a new group with additional middleware.
func (r *NetHTTPRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(r.middlewares)+len(middleware))
	mws = append(mws, r.middlewares...)
	mws = append(mws, middleware...)
	return &NetHTTPRouter{
		mux:                   r.mux,
		middlewares:           mws,
		notFound:              r.notFound,
		methodNA:              r.methodNA,
		responseWriterFactory: r.responseWriterFactory,
		registry:              r.registry,
		respHandler:           r.respHandler,
	}
}

// Handle registers a handler for the given method and pattern.
// Uses Go 1.22+ ServeMux pattern syntax: "METHOD /path".
func (r *NetHTTPRouter) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}

	// Go 1.22+ ServeMux supports {id} syntax directly
	muxPattern := method + " " + pattern
	wrapped := r.wrapHandler(handler, r.middlewares)
	r.mux.HandleFunc(muxPattern, wrapped)
	return nil
}

// Get registers a GET handler.
func (r *NetHTTPRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

// Post registers a POST handler.
func (r *NetHTTPRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

// Put registers a PUT handler.
func (r *NetHTTPRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

// Patch registers a PATCH handler.
func (r *NetHTTPRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

// Delete registers a DELETE handler.
func (r *NetHTTPRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

// Head registers a HEAD handler.
func (r *NetHTTPRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *NetHTTPRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
	r.mux.HandleFunc("/", r.wrapHandler(handler, r.middlewares))
}

// MethodNotAllowed sets the handler for disallowed methods.
// Go 1.22+ ServeMux handles 405 automatically; this stores the handler
// for manual dispatch if needed.
func (r *NetHTTPRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
}

// wrapHandler converts a v2 routing.Handler to an http.HandlerFunc.
func (r *NetHTTPRouter) wrapHandler(h routing.Handler, mws []routing.Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parser := r.responseWriterFactory(req, w)
		commit := &v2wf.CommitState{}

		// LegacyContext must be a context.Context carrying the
		// request and response writer via libNetHttp.WithRequestResponse,
		// so that libContext.InitContext follows the net/http branch.
		legacyCtx := legacyLibNetHttp.WithRequestResponse(req.Context(), req, w)

		reqCtx := &v2wf.RequestContext{
			Context:       req.Context(),
			LegacyContext: legacyCtx,
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}
		reqCtx.SetCommitState(commit)

		// Apply middleware chain
		chain := h
		for i := len(mws) - 1; i >= 0; i-- {
			chain = mws[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			if !commit.Committed() {
				r.dispatchError(reqCtx, err)
			}
		}
	}
}

// dispatchError routes an error through the v2 response registry if one is
// configured; otherwise it falls back to a sanitized 500 response.
func (r *NetHTTPRouter) dispatchError(ctx *v2wf.RequestContext, err error) {
	if r.respHandler != nil {
		_ = r.respHandler.Error(ctx, err)
		if ctx.Committed() {
			return
		}
	}
	_ = ctx.Parser.SendResponse(500, "application/json",
		[]byte(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`))
	ctx.MarkCommitted(500)
}

// netHTTPSubGroup implements RouteGroup for a prefixed sub-group.
type netHTTPSubGroup struct {
	parent      *NetHTTPRouter
	prefix      string
	middlewares []routing.Middleware
}

func (g *netHTTPSubGroup) Group(prefix string) routing.RouteGroup {
	return &netHTTPSubGroup{
		parent:      g.parent,
		prefix:      routing.JoinPath(g.prefix, prefix),
		middlewares: g.middlewares,
	}
}

func (g *netHTTPSubGroup) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(g.middlewares)+len(middleware))
	mws = append(mws, g.middlewares...)
	mws = append(mws, middleware...)
	return &netHTTPSubGroup{
		parent:      g.parent,
		prefix:      g.prefix,
		middlewares: mws,
	}
}

func (g *netHTTPSubGroup) Handle(method, pattern string, handler routing.Handler) error {
	fullPattern := routing.JoinPath(g.prefix, pattern)
	if err := routing.ValidatePattern(fullPattern); err != nil {
		return err
	}

	muxPattern := method + " " + fullPattern

	// Combine parent and group middleware
	allMws := make([]routing.Middleware, 0, len(g.parent.middlewares)+len(g.middlewares))
	allMws = append(allMws, g.parent.middlewares...)
	allMws = append(allMws, g.middlewares...)

	wrapped := g.parent.wrapHandler(handler, allMws)
	g.parent.mux.HandleFunc(muxPattern, wrapped)
	return nil
}

func (g *netHTTPSubGroup) Get(pattern string, handler routing.Handler) error {
	return g.Handle("GET", pattern, handler)
}

func (g *netHTTPSubGroup) Post(pattern string, handler routing.Handler) error {
	return g.Handle("POST", pattern, handler)
}

func (g *netHTTPSubGroup) Put(pattern string, handler routing.Handler) error {
	return g.Handle("PUT", pattern, handler)
}

func (g *netHTTPSubGroup) Patch(pattern string, handler routing.Handler) error {
	return g.Handle("PATCH", pattern, handler)
}

func (g *netHTTPSubGroup) Delete(pattern string, handler routing.Handler) error {
	return g.Handle("DELETE", pattern, handler)
}

func (g *netHTTPSubGroup) Head(pattern string, handler routing.Handler) error {
	return g.Handle("HEAD", pattern, handler)
}

// Ensure NetHTTPRouter implements routing.Router.
var _ routing.Router = (*NetHTTPRouter)(nil)

// Ensure netHTTPSubGroup implements routing.RouteGroup.
var _ routing.RouteGroup = (*netHTTPSubGroup)(nil)

// GetHTTPRequest extracts the *http.Request from a LegacyContext created by
// NetHTTPRouter. The LegacyContext is a context.Context carrying the request
// and response writer via libNetHttp.WithRequestResponse.
func GetHTTPRequest(ctx any) *http.Request {
	if c, ok := ctx.(context.Context); ok {
		if req, okReq := legacyLibNetHttp.RequestFromContext(c); okReq {
			return req
		}
	}
	return nil
}

// GetHTTPResponseWriter extracts the http.ResponseWriter from a LegacyContext
// created by NetHTTPRouter.
func GetHTTPResponseWriter(ctx any) http.ResponseWriter {
	if c, ok := ctx.(context.Context); ok {
		if w, okW := legacyLibNetHttp.ResponseWriterFromContext(c); okW {
			return w
		}
	}
	return nil
}

// Suppress unused import warning.
var _ = context.Background
