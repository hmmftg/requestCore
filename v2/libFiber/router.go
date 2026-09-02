// Package libFiber provides the v2 Fiber framework adapter for
// requestCore. It constructs request.Context and routing.Transport from
// fiber.Ctx and registers handlers with the Fiber app.
package libFiber

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// fiberTransport implements routing.Transport using a fiber.Ctx.
type fiberTransport struct {
	c         *fiber.Ctx
	committed bool
}

func (t *fiberTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	if t.committed {
		return nil
	}
	t.committed = true
	for k, vs := range headers {
		for _, v := range vs {
			t.c.Set(k, v)
		}
	}
	if contentType != "" {
		t.c.Set("Content-Type", contentType)
	}
	t.c.Status(status)
	return t.c.Send(body)
}

func (t *fiberTransport) Committed() bool {
	return t.committed || t.c.Response().StatusCode() != 0 && len(t.c.Response().Body()) > 0
}

// FiberRouter implements routing.Router and routing.RouteGroup for Fiber.
type FiberRouter struct {
	app                *fiber.App
	group              fiber.Router
	middlewares        []routing.Middleware
	errorHandler       routing.ErrorHandler
	notFound           routing.Handler
	methodNA           routing.Handler
	registeredRoutes   []routeEntry
	catchAllRegistered bool
}

type routeEntry struct {
	method  string
	pattern string
}

// NewRouter creates a new FiberRouter from a Fiber app.
func NewRouter(app *fiber.App) *FiberRouter {
	return &FiberRouter{app: app, group: app}
}

// NewRouterFromGroup creates a new FiberRouter from an existing Fiber group.
func NewRouterFromGroup(app *fiber.App, group fiber.Router) *FiberRouter {
	return &FiberRouter{app: app, group: group}
}

// SetErrorHandler sets the error handler for centralized error dispatch.
func (r *FiberRouter) SetErrorHandler(handler routing.ErrorHandler) {
	r.errorHandler = handler
}

// Native returns the underlying Fiber app.
func (r *FiberRouter) Native() any { return r.app }

// Group creates a sub-group with the given prefix.
func (r *FiberRouter) Group(prefix string) routing.RouteGroup {
	return &FiberRouter{
		app:          r.app,
		group:        r.group.Group(prefix),
		middlewares:  r.middlewares,
		errorHandler: r.errorHandler,
	}
}

// With returns a new group with additional middleware.
func (r *FiberRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(r.middlewares)+len(middleware))
	mws = append(mws, r.middlewares...)
	mws = append(mws, middleware...)
	return &FiberRouter{
		app:          r.app,
		group:        r.group,
		middlewares:  mws,
		errorHandler: r.errorHandler,
	}
}

// Handle registers a handler for the given method and pattern.
func (r *FiberRouter) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}
	fiberPattern := routing.TranslatePattern(pattern, "fiber")
	wrapped := r.wrapHandler(handler)
	r.group.Add(method, fiberPattern, wrapped)
	r.registeredRoutes = append(r.registeredRoutes, routeEntry{method: method, pattern: fiberPattern})
	return nil
}

func (r *FiberRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

func (r *FiberRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

func (r *FiberRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

func (r *FiberRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

func (r *FiberRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

func (r *FiberRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *FiberRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
	r.registerCatchAll()
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *FiberRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
	r.registerCatchAll()
}

func (r *FiberRouter) registerCatchAll() {
	if r.catchAllRegistered {
		return
	}
	r.catchAllRegistered = true
	r.app.Use("*", func(c *fiber.Ctx) error {
		if err := c.Next(); err != nil {
			return err
		}
		if c.Response().StatusCode() != 404 {
			return nil
		}
		path := c.Path()
		method := c.Method()
		pathMatches := false
		for _, entry := range r.registeredRoutes {
			if fiberPathMatches(entry.pattern, path) {
				if entry.method == method {
					pathMatches = false
					break
				}
				pathMatches = true
			}
		}
		var handler routing.Handler
		if pathMatches && r.methodNA != nil {
			handler = r.methodNA
		} else if r.notFound != nil {
			handler = r.notFound
		}
		if handler == nil {
			return nil
		}
		ctx := buildContext(c)
		transport := &fiberTransport{c: c}
		if err := handler(ctx, transport); err != nil {
			if !transport.Committed() && r.errorHandler != nil {
				r.errorHandler(ctx, transport, err)
			}
		}
		return nil
	})
}

func fiberPathMatches(pattern, path string) bool {
	if pattern == path {
		return true
	}
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i, pp := range patternParts {
		if strings.HasPrefix(pp, ":") || strings.HasPrefix(pp, "*") {
			continue
		}
		if pp != pathParts[i] {
			return false
		}
	}
	return true
}

func splitPath(p string) []string {
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

// buildContext creates a *request.Context from a fiber.Ctx.
func buildContext(c *fiber.Ctx) *request.Context {
	// Extract path parameters from the matched route.
	pathParams := make(map[string]string)
	if route := c.Route(); route != nil {
		for _, name := range route.Params {
			if val := c.Params(name); val != "" {
				pathParams[name] = val
			}
		}
	}
	// Convert fiber's map[string]string to url.Values.
	queries := c.Queries()
	qv := make(url.Values, len(queries))
	for k, v := range queries {
		qv[k] = []string{v}
	}
	opts := []request.Option{
		request.WithMethod(c.Method()),
		request.WithPath(c.Path()),
		request.WithHeader(c.GetReqHeaders()),
		request.WithQuery(qv),
		request.WithRemoteAddr(c.IP()),
		request.WithNative(c),
	}
	if len(pathParams) > 0 {
		opts = append(opts, request.WithPathParams(pathParams))
	}
	body := c.Body()
	if len(body) > 0 {
		opts = append(opts, request.WithBodySource(request.NewStringBodySource(string(body))))
	}
	return request.NewContext(c.UserContext(), opts...)
}

// wrapHandler converts a routing.Handler to a fiber.Handler.
func (r *FiberRouter) wrapHandler(h routing.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := buildContext(c)
		transport := &fiberTransport{c: c}
		chain := h
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			chain = r.middlewares[i](chain)
		}
		if err := chain(ctx, transport); err != nil {
			if !transport.Committed() && r.errorHandler != nil {
				r.errorHandler(ctx, transport, err)
			}
		}
		return nil
	}
}

// Ensure interface implementations.
var _ routing.Router = (*FiberRouter)(nil)
var _ routing.RouteGroup = (*FiberRouter)(nil)
var _ routing.Transport = (*fiberTransport)(nil)
