// Package libNetHttp provides the v2 net/http framework adapter for
// requestCore. It constructs request.Context and routing.Transport from
// standard net/http types and registers handlers with http.ServeMux.
//
// This adapter is stdlib-only: it does not import webFramework or any
// v1 package.
package libNetHttp

import (
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// netHTTPTransport implements routing.Transport using an
// http.ResponseWriter. It tracks commit state to prevent double-writes.
type netHTTPTransport struct {
	w         http.ResponseWriter
	committed bool
}

// WriteResponse writes the HTTP response with the given status, content
// type, headers, and body. Headers are applied with add semantics to
// preserve multi-value headers like Set-Cookie. Once committed,
// subsequent calls are no-ops.
func (t *netHTTPTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
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

// Committed reports whether the response has been committed.
func (t *netHTTPTransport) Committed() bool {
	return t.committed
}

// NetHTTPRouter implements routing.Router for net/http using http.ServeMux.
type NetHTTPRouter struct {
	mux              *http.ServeMux
	middlewares      []routing.Middleware
	notFound         routing.Handler
	methodNA         routing.Handler
	errorHandler     routing.ErrorHandler
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
		mux:              http.NewServeMux(),
		registeredRoutes: &routes,
	}
}

// NewRouterFromMux creates a new NetHTTPRouter from an existing http.ServeMux.
func NewRouterFromMux(mux *http.ServeMux) *NetHTTPRouter {
	routes := make([]routeEntry, 0)
	return &NetHTTPRouter{
		mux:              mux,
		registeredRoutes: &routes,
	}
}

// SetErrorHandler sets the error handler for centralized error dispatch.
func (r *NetHTTPRouter) SetErrorHandler(handler routing.ErrorHandler) {
	r.errorHandler = handler
}

// Native returns the underlying http.ServeMux, wrapped with 404/405
// interception if a NotFound or MethodNotAllowed handler has been
// configured.
func (r *NetHTTPRouter) Native() any {
	if r.notFound != nil || r.methodNA != nil {
		return r.intercept404_405(r.mux)
	}
	return r.mux
}

// Group creates a sub-group with the given prefix.
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
		mux:              r.mux,
		middlewares:      mws,
		notFound:         r.notFound,
		methodNA:         r.methodNA,
		errorHandler:     r.errorHandler,
		registeredRoutes: r.registeredRoutes,
	}
}

// Handle registers a handler for the given method and pattern.
func (r *NetHTTPRouter) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}
	muxPattern := method + " " + pattern
	wrapped := r.wrapHandler(handler, r.middlewares, pattern)
	r.mux.HandleFunc(muxPattern, wrapped)
	*r.registeredRoutes = append(*r.registeredRoutes, routeEntry{method: method, pattern: pattern})
	return nil
}

func (r *NetHTTPRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

func (r *NetHTTPRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

func (r *NetHTTPRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

func (r *NetHTTPRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

func (r *NetHTTPRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

func (r *NetHTTPRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *NetHTTPRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *NetHTTPRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
}

// buildContext creates a *request.Context from an *http.Request,
// extracting path parameters from Go 1.22+ ServeMux's PathValue.
// The pattern is used to determine which path parameters to extract;
// an empty pattern means no parameters are extracted (e.g. for 404/405
// handlers dispatched by the interception layer).
func buildContext(req *http.Request, pattern string) *request.Context {
	opts := []request.Option{
		request.WithMethod(req.Method),
		request.WithPath(req.URL.Path),
		request.WithHeader(req.Header),
		request.WithQuery(req.URL.Query()),
		request.WithRemoteAddr(req.RemoteAddr),
		request.WithNative(req),
		request.WithBodySource(request.NewBodySource(req.Body)),
	}
	// Extract path parameters from Go 1.22+ ServeMux using the
	// registered pattern to know which param names to look up.
	pathParams := make(map[string]string)
	for _, name := range extractParamNames(pattern) {
		if v := req.PathValue(name); v != "" {
			pathParams[name] = v
		}
	}
	if len(pathParams) > 0 {
		opts = append(opts, request.WithPathParams(pathParams))
	}
	// Extract cookies.
	if len(req.Cookies()) > 0 {
		opts = append(opts, request.WithCookies(req.Cookies()))
	}
	return request.NewContext(req.Context(), opts...)
}

// extractParamNames extracts parameter names from a canonical v2
// pattern (e.g. "/users/{id}" -> ["id"], "/files/{path...}" -> ["path"]).
func extractParamNames(pattern string) []string {
	var names []string
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '{' {
			end := strings.Index(pattern[i:], "}")
			if end == -1 {
				break
			}
			name := pattern[i+1 : i+end]
			// Strip "..." suffix for wildcard params.
			name = strings.TrimSuffix(name, "...")
			if name != "" {
				names = append(names, name)
			}
			i += end
		}
	}
	return names
}

// wrapHandler converts a routing.Handler to an http.HandlerFunc.
// The pattern is used to extract path parameters from Go 1.22+ ServeMux.
func (r *NetHTTPRouter) wrapHandler(h routing.Handler, mws []routing.Middleware, pattern string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := buildContext(req, pattern)
		transport := &netHTTPTransport{w: w}

		// Apply middleware chain.
		chain := h
		for i := len(mws) - 1; i >= 0; i-- {
			chain = mws[i](chain)
		}

		if err := chain(ctx, transport); err != nil {
			if !transport.Committed() && r.errorHandler != nil {
				r.errorHandler(ctx, transport, err)
			}
		}
	}
}

// intercept404_405 wraps the mux handler to intercept 404 and 405
// responses from Go 1.22+ ServeMux.
func (r *NetHTTPRouter) intercept404_405(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Use a tracking response writer to detect 404/405.
		tracker := &trackingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(tracker, req)

		var handler routing.Handler
		switch tracker.status {
		case http.StatusMethodNotAllowed:
			if r.methodNA != nil {
				handler = r.methodNA
			}
		case http.StatusNotFound:
			if r.methodNA != nil && pathMatchesDifferentMethod(*r.registeredRoutes, req.URL.Path, req.Method) {
				handler = r.methodNA
			} else if r.notFound != nil {
				handler = r.notFound
			}
		}

		if handler == nil {
			// Forward the buffered response to the real writer.
			if tracker.headerSent {
				w.WriteHeader(tracker.status)
			}
			_, _ = w.Write([]byte(tracker.body.String()))
			return
		}

		// Dispatch through the v2 pipeline with the real writer.
		ctx := buildContext(req, "")
		transport := &netHTTPTransport{w: w}
		if err := handler(ctx, transport); err != nil {
			if !transport.Committed() && r.errorHandler != nil {
				r.errorHandler(ctx, transport, err)
			}
		}
	})
}

// trackingResponseWriter captures the status code and body for 404/405
// interception without requiring httptest.NewRecorder. It buffers all
// writes so that the interception layer can decide whether to forward
// the buffered response or dispatch a custom handler with the clean
// real writer.
type trackingResponseWriter struct {
	http.ResponseWriter
	status     int
	headerSent bool
	body       strings.Builder
}

func (t *trackingResponseWriter) WriteHeader(status int) {
	t.status = status
	t.headerSent = true
}

func (t *trackingResponseWriter) Write(b []byte) (int, error) {
	if !t.headerSent {
		t.status = 200
		t.headerSent = true
	}
	t.body.Write(b)
	return len(b), nil
}

// pathMatchesDifferentMethod checks if the given path matches a
// registered route pattern with a method different from the request.
func pathMatchesDifferentMethod(routes []routeEntry, path, method string) bool {
	for _, entry := range routes {
		if netHTTPPathMatches(entry.pattern, path) && entry.method != method {
			return true
		}
	}
	return false
}

// netHTTPPathMatches checks if a Go 1.22+ ServeMux pattern matches the
// given path.
func netHTTPPathMatches(pattern, path string) bool {
	if idx := strings.Index(pattern, " "); idx >= 0 {
		pattern = pattern[idx+1:]
	}
	if pattern == path {
		return true
	}
	patternParts := splitNetHTTPPath(pattern)
	pathParts := splitNetHTTPPath(path)
	for i, pp := range patternParts {
		if i >= len(pathParts) {
			return false
		}
		if strings.HasSuffix(pp, "...}") {
			return true
		}
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}
	return len(patternParts) == len(pathParts)
}

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
	allMws := make([]routing.Middleware, 0, len(g.parent.middlewares)+len(g.middlewares))
	allMws = append(allMws, g.parent.middlewares...)
	allMws = append(allMws, g.middlewares...)
	wrapped := g.parent.wrapHandler(handler, allMws, fullPattern)
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

// Ensure interface implementations.
var _ routing.Router = (*NetHTTPRouter)(nil)
var _ routing.RouteGroup = (*netHTTPSubGroup)(nil)
var _ routing.Transport = (*netHTTPTransport)(nil)
