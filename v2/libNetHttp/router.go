package libNetHttp

import (
	"context"
	"net/http"

	legacy "github.com/hmmftg/requestCore/webFramework"

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
	wrapped := r.wrapHandler(handler)
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
	r.mux.HandleFunc("/", r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
// Go 1.22+ ServeMux handles this automatically.
func (r *NetHTTPRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
}

// wrapHandler converts a v2 routing.Handler to an http.HandlerFunc.
func (r *NetHTTPRouter) wrapHandler(h routing.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parser := r.responseWriterFactory(req, w)

		reqCtx := &v2wf.RequestContext{
			Context:       req.Context(),
			LegacyContext: context.WithValue(req.Context(), httpRequestKey{}, req),
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}

		// Apply middleware chain
		chain := h
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			chain = r.middlewares[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			// If no response was written, send 500
			if !responseWritten(w) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				_, _ = w.Write([]byte(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`))
			}
		}
	}
}

// responseWritten checks if the response writer has had WriteHeader called.
type writtenChecker interface {
	Written() bool
}

func responseWritten(w http.ResponseWriter) bool {
	if wc, ok := w.(writtenChecker); ok {
		return wc.Written()
	}
	// Default: assume not written if we can't check
	return false
}

// httpRequestKey is used to store the *http.Request in the LegacyContext.
type httpRequestKey struct{}

// HTTPRequestKey is the exported version of httpRequestKey for use by
// other adapters (e.g. libChi) that need to extract the *http.Request
// from the LegacyContext.
type HTTPRequestKey = httpRequestKey

// GetHTTPRequest extracts the *http.Request from a LegacyContext created by NetHTTPRouter.
func GetHTTPRequest(ctx context.Context) *http.Request {
	if v, ok := ctx.Value(httpRequestKey{}).(*http.Request); ok {
		return v
	}
	return nil
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

	wrapped := g.wrapHandlerWithMw(handler, allMws)
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

func (g *netHTTPSubGroup) wrapHandlerWithMw(h routing.Handler, mws []routing.Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parser := g.parent.responseWriterFactory(req, w)
		reqCtx := &v2wf.RequestContext{
			Context:       req.Context(),
			LegacyContext: context.WithValue(req.Context(), httpRequestKey{}, req),
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}

		chain := h
		for i := len(mws) - 1; i >= 0; i-- {
			chain = mws[i](chain)
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

// Ensure NetHTTPRouter implements routing.Router.
var _ routing.Router = (*NetHTTPRouter)(nil)

// Ensure netHTTPSubGroup implements routing.RouteGroup.
var _ routing.RouteGroup = (*netHTTPSubGroup)(nil)

// Suppress unused import warning.
var _ = context.Background
