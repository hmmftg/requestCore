// Package libChi provides the v2 chi web framework adapter for
// requestCore. It constructs request.Context and routing.Transport from
// net/http types and chi's route context, and registers handlers with
// the chi.Mux.
package libChi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// chiTransport implements routing.Transport using an http.ResponseWriter.
type chiTransport struct {
	w         http.ResponseWriter
	committed bool
}

func (t *chiTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	if t.committed {
		return nil
	}
	t.committed = true
	for k, vs := range headers {
		for _, v := range vs {
			t.w.Header().Add(k, v)
		}
	}
	if contentType != "" {
		t.w.Header().Set("Content-Type", contentType)
	}
	t.w.WriteHeader(status)
	_, err := t.w.Write(body)
	return err
}

func (t *chiTransport) Committed() bool {
	return t.committed
}

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

// SetErrorHandler sets the error handler for centralized error dispatch.
func (r *ChiRouter) SetErrorHandler(handler routing.ErrorHandler) {
	r.routeGroup.errorHandler = handler
}

// Native returns the underlying chi.Mux.
func (r *ChiRouter) Native() any { return r.mux }

func (r *ChiRouter) Group(prefix string) routing.RouteGroup {
	return r.routeGroup.Group(prefix)
}

func (r *ChiRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	return r.routeGroup.With(middleware...)
}

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

func (r *ChiRouter) NotFound(handler routing.Handler) {
	r.mux.NotFound(r.routeGroup.wrapHandler(handler))
}

func (r *ChiRouter) MethodNotAllowed(handler routing.Handler) {
	r.mux.MethodNotAllowed(r.routeGroup.wrapHandler(handler))
}

// chiGroup implements routing.RouteGroup for chi.
type chiGroup struct {
	mux          *chi.Mux
	r            chi.Router
	middlewares  []routing.Middleware
	errorHandler routing.ErrorHandler
}

func (g *chiGroup) Group(prefix string) routing.RouteGroup {
	var sub chi.Router
	g.r.Route(prefix, func(r chi.Router) {
		sub = r
	})
	if sub == nil {
		sub = g.r
	}
	return &chiGroup{mux: g.mux, r: sub, middlewares: g.middlewares, errorHandler: g.errorHandler}
}

func (g *chiGroup) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(g.middlewares)+len(middleware))
	mws = append(mws, g.middlewares...)
	mws = append(mws, middleware...)
	return &chiGroup{mux: g.mux, r: g.r, middlewares: mws, errorHandler: g.errorHandler}
}

func (g *chiGroup) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}
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

// buildContext creates a *request.Context from an *http.Request,
// extracting path parameters from chi's route context.
func buildContext(req *http.Request) *request.Context {
	pathParams := make(map[string]string)
	if chiCtx := chi.RouteContext(req.Context()); chiCtx != nil {
		for i, key := range chiCtx.URLParams.Keys {
			if i < len(chiCtx.URLParams.Values) {
				pathParams[key] = chiCtx.URLParams.Values[i]
			}
		}
	}
	opts := []request.Option{
		request.WithMethod(req.Method),
		request.WithPath(req.URL.Path),
		request.WithHeader(req.Header),
		request.WithQuery(req.URL.Query()),
		request.WithRemoteAddr(req.RemoteAddr),
		request.WithNative(req),
		request.WithBodySource(request.NewBodySource(req.Body)),
	}
	if len(pathParams) > 0 {
		opts = append(opts, request.WithPathParams(pathParams))
	}
	if len(req.Cookies()) > 0 {
		opts = append(opts, request.WithCookies(req.Cookies()))
	}
	return request.NewContext(req.Context(), opts...)
}

func (g *chiGroup) wrapHandler(h routing.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := buildContext(req)
		transport := &chiTransport{w: w}
		chain := h
		for i := len(g.middlewares) - 1; i >= 0; i-- {
			chain = g.middlewares[i](chain)
		}
		if err := chain(ctx, transport); err != nil {
			if !transport.Committed() && g.errorHandler != nil {
				g.errorHandler(ctx, transport, err)
			}
		}
	}
}

// Ensure interface implementations.
var _ routing.Router = (*ChiRouter)(nil)
var _ routing.RouteGroup = (*chiGroup)(nil)
var _ routing.Transport = (*chiTransport)(nil)
