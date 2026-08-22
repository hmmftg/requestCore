// Package libChi provides the v2 chi web framework adapter for requestCore.
//
// chi is used as the default router for the net/http v2 adapter, providing
// richer routing features (middleware, groups, path parameters) than the
// standard http.ServeMux.
package libChi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
	legacy "github.com/hmmftg/requestCore/webFramework"
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
	return &chiGroup{mux: g.mux, r: sub, middlewares: g.middlewares}
}

func (g *chiGroup) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(g.middlewares)+len(middleware))
	mws = append(mws, g.middlewares...)
	mws = append(mws, middleware...)
	return &chiGroup{mux: g.mux, r: g.r, middlewares: mws}
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

		// Populate Params from chi URL params
		if chiCtx := chi.RouteContext(req.Context()); chiCtx != nil {
			for i, key := range chiCtx.URLParams.Keys {
				if i < len(chiCtx.URLParams.Values) {
					parser.Params[key] = chiCtx.URLParams.Values[i]
				}
			}
		}

		reqCtx := &v2wf.RequestContext{
			Context:       req.Context(),
			LegacyContext: context.WithValue(req.Context(), v2libNetHttp.HTTPRequestKey{}, req),
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}

		// Apply middleware chain
		chain := h
		for i := len(g.middlewares) - 1; i >= 0; i-- {
			chain = g.middlewares[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			if !responseWritten(w) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`))
			}
		}
	}
}

type writtenChecker interface {
	Written() bool
}

func responseWritten(w http.ResponseWriter) bool {
	if wc, ok := w.(writtenChecker); ok {
		return wc.Written()
	}
	return false
}

// Ensure ChiRouter implements routing.Router.
var _ routing.Router = (*ChiRouter)(nil)

// Ensure chiGroup implements routing.RouteGroup.
var _ routing.RouteGroup = (*chiGroup)(nil)
