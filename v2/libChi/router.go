// Package libChi provides the v2 chi web framework adapter for requestCore.
//
// chi is used as the default router for the net/http v2 adapter, providing
// richer routing features (middleware, groups, path parameters) than the
// standard http.ServeMux.
package libChi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	legacyLibNetHttp "github.com/hmmftg/requestCore/libNetHttp"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// ChiRouter implements routing.Router using chi.
type ChiRouter struct {
	mux        *chi.Mux
	routeGroup *chiGroup
}

// NewRouter creates a new ChiRouter with a new chi.Mux.
func NewRouter() *ChiRouter {
	mux := chi.NewRouter()
	r := &ChiRouter{mux: mux}
	r.routeGroup = &chiGroup{mux: mux, r: mux, middlewares: nil}
	return r
}

// NewRouterFromMux creates a new ChiRouter from an existing chi.Mux.
func NewRouterFromMux(mux *chi.Mux) *ChiRouter {
	r := &ChiRouter{mux: mux}
	r.routeGroup = &chiGroup{mux: mux, r: mux, middlewares: nil}
	return r
}

// SetErrorHandler installs the v2 response handler and error registry used
// for centralized error dispatch.
func (r *ChiRouter) SetErrorHandler(handler *response.Handler) {
	if handler == nil {
		return
	}
	r.routeGroup.respHandler = handler
	r.routeGroup.registry = handler.Registry()
}

// Native returns the underlying chi.Mux.
func (r *ChiRouter) Native() any {
	return r.mux
}

// Group creates a sub-group with the given prefix.
func (r *ChiRouter) Group(prefix string) routing.RouteGroup {
	return r.routeGroup.Group(prefix)
}

// With returns a new group with additional middleware.
func (r *ChiRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	return r.routeGroup.With(middleware...)
}

// Handle registers a handler for the given method and pattern.
func (r *ChiRouter) Handle(method, pattern string, handler routing.Handler) error {
	return r.routeGroup.Handle(method, pattern, handler)
}

func (r *ChiRouter) Get(pattern string, handler routing.Handler) error {
	return r.routeGroup.Get(pattern, handler)
}

func (r *ChiRouter) Post(pattern string, handler routing.Handler) error {
	return r.routeGroup.Post(pattern, handler)
}

func (r *ChiRouter) Put(pattern string, handler routing.Handler) error {
	return r.routeGroup.Put(pattern, handler)
}

func (r *ChiRouter) Patch(pattern string, handler routing.Handler) error {
	return r.routeGroup.Patch(pattern, handler)
}

func (r *ChiRouter) Delete(pattern string, handler routing.Handler) error {
	return r.routeGroup.Delete(pattern, handler)
}

func (r *ChiRouter) Head(pattern string, handler routing.Handler) error {
	return r.routeGroup.Head(pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *ChiRouter) NotFound(handler routing.Handler) {
	r.mux.NotFound(r.routeGroup.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *ChiRouter) MethodNotAllowed(handler routing.Handler) {
	r.mux.MethodNotAllowed(r.routeGroup.wrapHandler(handler))
}

// chiGroup implements routing.RouteGroup for chi.
type chiGroup struct {
	mux         *chi.Mux
	r           chi.Router
	middlewares []routing.Middleware
	registry    response.Registry
	respHandler *response.Handler
}

func (g *chiGroup) Group(prefix string) routing.RouteGroup {
	// chi's Group takes a function, not a prefix.
	// We use Route for prefix-based grouping.
	var sub chi.Router
	g.r.Route(prefix, func(r chi.Router) {
		sub = r
	})
	if sub == nil {
		sub = g.r
	}
	return &chiGroup{mux: g.mux, r: sub, middlewares: g.middlewares, registry: g.registry, respHandler: g.respHandler}
}

func (g *chiGroup) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(g.middlewares)+len(middleware))
	mws = append(mws, g.middlewares...)
	mws = append(mws, middleware...)
	return &chiGroup{mux: g.mux, r: g.r, middlewares: mws, registry: g.registry, respHandler: g.respHandler}
}

func (g *chiGroup) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}

	// chi uses {id} syntax directly
	wrapped := g.wrapHandler(handler)
	g.r.Method(method, pattern, http.HandlerFunc(wrapped))
	return nil
}

func (g *chiGroup) Get(pattern string, handler routing.Handler) error {
	return g.Handle("GET", pattern, handler)
}

func (g *chiGroup) Post(pattern string, handler routing.Handler) error {
	return g.Handle("POST", pattern, handler)
}

func (g *chiGroup) Put(pattern string, handler routing.Handler) error {
	return g.Handle("PUT", pattern, handler)
}

func (g *chiGroup) Patch(pattern string, handler routing.Handler) error {
	return g.Handle("PATCH", pattern, handler)
}

func (g *chiGroup) Delete(pattern string, handler routing.Handler) error {
	return g.Handle("DELETE", pattern, handler)
}

func (g *chiGroup) Head(pattern string, handler routing.Handler) error {
	return g.Handle("HEAD", pattern, handler)
}

func (g *chiGroup) wrapHandler(h routing.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parser := v2libNetHttp.InitContextV2(req, w)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)

		// Populate Params from chi URL params
		if chiCtx := chi.RouteContext(req.Context()); chiCtx != nil {
			for i, key := range chiCtx.URLParams.Keys {
				if i < len(chiCtx.URLParams.Values) {
					parser.Params[key] = chiCtx.URLParams.Values[i]
				}
			}
		}

		// LegacyContext must carry the request and response writer
		// via libNetHttp.WithRequestResponse so libContext.InitContext
		// follows the net/http branch.
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
		for i := len(g.middlewares) - 1; i >= 0; i-- {
			chain = g.middlewares[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			if !commit.Committed() {
				g.dispatchError(reqCtx, err)
			}
		}
	}
}

// dispatchError routes an error through the shared adapter error-dispatch
// helper, which uses the v2 response registry if configured and falls back
// to a sanitized 500 response.
func (g *chiGroup) dispatchError(ctx *v2wf.RequestContext, err error) {
	response.DispatchError(g.respHandler, ctx, err)
}

// Ensure ChiRouter implements routing.Router.
var _ routing.Router = (*ChiRouter)(nil)

// Ensure chiGroup implements routing.RouteGroup.
var _ routing.RouteGroup = (*chiGroup)(nil)
