package libNetHttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

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
	// registeredRoutes tracks method+pattern pairs for 405 dispatch when
	// a catch-all NotFound handler would otherwise shadow method mismatches.
	// It is a pointer so that With-derived routers share the same slice
	// as the root router, ensuring all routes are visible to 405 detection.
	registeredRoutes *[]routeEntry
}

// routeEntry records a registered method and pattern for 405 checks.
type routeEntry struct {
	method  string
	pattern string
}

// NewRouter creates a new NetHTTPRouter with a new http.ServeMux.
func NewRouter() *NetHTTPRouter {
	routes := make([]routeEntry, 0)
	return &NetHTTPRouter{
		mux:                   http.NewServeMux(),
		responseWriterFactory: InitContextV2,
		registeredRoutes:      &routes,
	}
}

// NewRouterFromMux creates a new NetHTTPRouter from an existing http.ServeMux.
func NewRouterFromMux(mux *http.ServeMux) *NetHTTPRouter {
	routes := make([]routeEntry, 0)
	return &NetHTTPRouter{
		mux:                   mux,
		responseWriterFactory: InitContextV2,
		registeredRoutes:      &routes,
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

// Native returns the underlying http.ServeMux, wrapped with 404/405
// interception if a NotFound or MethodNotAllowed handler has been configured.
func (r *NetHTTPRouter) Native() any {
	if r.notFound != nil || r.methodNA != nil {
		return r.intercept404_405(r.mux)
	}
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
		registeredRoutes:      r.registeredRoutes,
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
	*r.registeredRoutes = append(*r.registeredRoutes, routeEntry{method: method, pattern: pattern})
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

// NotFound sets the handler for unmatched routes. The handler is dispatched
// by the intercept404_405 wrapper returned by Native(); it is NOT registered
// as a catch-all on the mux because that would shadow Go 1.22+ ServeMux's
// automatic 405 responses for method mismatches on registered patterns.
func (r *NetHTTPRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
}

// MethodNotAllowed sets the handler for disallowed methods. The handler is
// dispatched by the intercept404_405 wrapper returned by Native().
func (r *NetHTTPRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
}

// intercept404_405 wraps the mux handler to intercept 404 and 405 responses
// from Go 1.22+ ServeMux and dispatch them through the v2 handler pipeline.
// A 405 is dispatched to the methodNA handler; a 404 is dispatched to the
// notFound handler. If only one of the two is configured, the other falls
// through to the mux's default response.
func (r *NetHTTPRouter) intercept404_405(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, req)

		var handler routing.Handler
		switch rec.Code {
		case http.StatusMethodNotAllowed:
			if r.methodNA != nil {
				handler = r.methodNA
			}
		case http.StatusNotFound:
			// Check if the path matches a registered route with a
			// different method → 405 takes precedence over 404.
			if r.methodNA != nil && pathMatchesDifferentMethod(*r.registeredRoutes, req.URL.Path, req.Method) {
				handler = r.methodNA
			} else if r.notFound != nil {
				handler = r.notFound
			}
		}

		if handler == nil {
			// Forward the buffered response to the real writer.
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
			return
		}

		// Dispatch through the v2 pipeline with the real ResponseWriter.
		parser := r.responseWriterFactory(req, w)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)
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
		if err := handler(reqCtx); err != nil {
			r.dispatchError(reqCtx, err)
		}
	})
}

// pathMatchesDifferentMethod checks if the given path matches a registered
// route pattern with a method different from the request method.
func pathMatchesDifferentMethod(routes []routeEntry, path, method string) bool {
	for _, entry := range routes {
		if netHTTPPathMatches(entry.pattern, path) && entry.method != method {
			return true
		}
	}
	return false
}

// netHTTPPathMatches checks if a Go 1.22+ ServeMux pattern matches the
// given path. The pattern may contain {param} single-segment wildcards
// and {param...} multi-segment wildcards.
func netHTTPPathMatches(pattern, path string) bool {
	// Strip method prefix if present.
	if idx := strings.Index(pattern, " "); idx >= 0 {
		pattern = pattern[idx+1:]
	}
	// Exact match.
	if pattern == path {
		return true
	}
	patternParts := splitNetHTTPPath(pattern)
	pathParts := splitNetHTTPPath(path)
	// Multi-segment wildcard {param...} matches the rest of the path,
	// so the pattern may have fewer segments than the path.
	for i, pp := range patternParts {
		if i >= len(pathParts) {
			return false
		}
		// Go 1.22+ ServeMux multi-segment wildcard {param...}.
		if strings.HasSuffix(pp, "...}") {
			return true
		}
		// Single-segment wildcard {param}.
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}
	return len(patternParts) == len(pathParts)
}

// splitNetHTTPPath splits a URL path into segments.
func splitNetHTTPPath(p string) []string {
	if len(p) == 0 {
		return nil
	}
	if p[0] == '/' {
		p = p[1:]
	}
	if len(p) == 0 {
		return nil
	}
	return strings.Split(p, "/")
}

// wrapHandler converts a v2 routing.Handler to an http.HandlerFunc.
func (r *NetHTTPRouter) wrapHandler(h routing.Handler, mws []routing.Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parser := r.responseWriterFactory(req, w)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)

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

// dispatchError routes an error through the shared adapter error-dispatch
// helper, which uses the v2 response registry if configured and falls back
// to a sanitized 500 response.
func (r *NetHTTPRouter) dispatchError(ctx *v2wf.RequestContext, err error) {
	response.DispatchError(r.respHandler, ctx, err)
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
	*g.parent.registeredRoutes = append(*g.parent.registeredRoutes, routeEntry{method: method, pattern: fullPattern})
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
