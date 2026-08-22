package libFiber

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// FiberRouter implements routing.Router and routing.RouteGroup for Fiber.
type FiberRouter struct {
	app         *fiber.App
	group       fiber.Router
	middlewares []routing.Middleware
	registry    response.Registry
	respHandler *response.Handler
	notFound    routing.Handler
	methodNA    routing.Handler
	// registeredRoutes tracks method+pattern pairs for 405 dispatch.
	registeredRoutes []routeEntry
	// catchAllRegistered prevents duplicate catch-all registration.
	catchAllRegistered bool
}

// routeEntry records a registered method and pattern for 405 checks.
type routeEntry struct {
	method  string
	pattern string
}

// NewRouter creates a new FiberRouter from a Fiber app.
func NewRouter(app *fiber.App) *FiberRouter {
	return &FiberRouter{
		app:   app,
		group: app,
	}
}

// NewRouterFromGroup creates a new FiberRouter from an existing Fiber group.
func NewRouterFromGroup(app *fiber.App, group fiber.Router) *FiberRouter {
	return &FiberRouter{
		app:   app,
		group: group,
	}
}

// SetErrorHandler installs the v2 response handler and error registry used
// for centralized error dispatch.
func (r *FiberRouter) SetErrorHandler(handler *response.Handler) {
	if handler == nil {
		return
	}
	r.respHandler = handler
	r.registry = handler.Registry()
}

// Native returns the underlying Fiber app.
func (r *FiberRouter) Native() any {
	return r.app
}

// Group creates a sub-group with the given prefix.
func (r *FiberRouter) Group(prefix string) routing.RouteGroup {
	return &FiberRouter{
		app:         r.app,
		group:       r.group.Group(prefix),
		middlewares: r.middlewares,
		registry:    r.registry,
		respHandler: r.respHandler,
	}
}

// With returns a new group with additional middleware.
func (r *FiberRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(r.middlewares)+len(middleware))
	mws = append(mws, r.middlewares...)
	mws = append(mws, middleware...)
	return &FiberRouter{
		app:         r.app,
		group:       r.group,
		middlewares: mws,
		registry:    r.registry,
		respHandler: r.respHandler,
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

// Get registers a GET handler.
func (r *FiberRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

// Post registers a POST handler.
func (r *FiberRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

// Put registers a PUT handler.
func (r *FiberRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

// Patch registers a PATCH handler.
func (r *FiberRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

// Delete registers a DELETE handler.
func (r *FiberRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

// Head registers a HEAD handler.
func (r *FiberRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes. It registers a single
// catch-all that dispatches either the 404 handler (no route matches the
// path) or the 405 handler (a route matches the path but with a different
// method), ensuring the two do not shadow each other.
func (r *FiberRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
	r.registerCatchAll()
}

// MethodNotAllowed sets the handler for disallowed methods. The handler is
// dispatched by the catch-all registered by NotFound (or by this method if
// NotFound was not called).
func (r *FiberRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
	r.registerCatchAll()
}

// registerCatchAll registers a single Fiber catch-all middleware that
// dispatches 404 or 405 based on whether the request path matches a
// registered route with a different method. It is idempotent: calling it
// multiple times has no effect beyond the first.
//
// The middleware calls c.Next() first to let route handlers run. After
// the handler chain completes, if the response status is still 404
// (Fiber's default for unmatched routes), the catch-all dispatches the
// configured 404 or 405 handler.
func (r *FiberRouter) registerCatchAll() {
	if r.catchAllRegistered {
		return
	}
	r.catchAllRegistered = true
	r.app.Use("*", func(c *fiber.Ctx) error {
		// Continue to the next handler (route handler or Fiber's
		// default 404). This ensures route handlers run before the
		// catch-all checks the response status.
		if err := c.Next(); err != nil {
			return err
		}
		// If a matched route already sent a response (status != 404),
		// skip the catch-all.
		if c.Response().StatusCode() != 404 {
			return nil
		}
		path := c.Path()
		method := c.Method()

		// Check if the path matches a registered route with a
		// different method → 405. If no route matches at all → 404.
		pathMatches := false
		for _, entry := range r.registeredRoutes {
			if fiberPathMatches(entry.pattern, path) {
				if entry.method == method {
					// Route matches exactly; should have been handled.
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

		// Dispatch through the v2 pipeline.
		parser := InitContextV2(c)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)
		reqCtx := &v2wf.RequestContext{
			LegacyContext: c,
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}
		reqCtx.SetCommitState(commit)
		reqCtx.Context = c.UserContext()
		if err := handler(reqCtx); err != nil {
			r.dispatchError(reqCtx, err)
		}
		return nil
	})
}

// fiberPathMatches checks if a Fiber pattern matches the given path.
// This is a simplified check that handles Fiber's parameter syntax.
func fiberPathMatches(pattern, path string) bool {
	// Exact match.
	if pattern == path {
		return true
	}
	// Handle parameter patterns like /users/:id.
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

// splitPath splits a URL path into segments.
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

// wrapHandler converts a v2 routing.Handler to a fiber.Handler,
// running the middleware chain and building the RequestContext.
func (r *FiberRouter) wrapHandler(h routing.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		parser := InitContextV2(c)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)
		// LegacyContext is the native *fiber.Ctx expected by
		// libContext.InitContext (it switches on *fiber.Ctx).
		reqCtx := &v2wf.RequestContext{
			LegacyContext: c,
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}
		reqCtx.SetCommitState(commit)
		reqCtx.Context = c.UserContext()

		// Apply middleware chain
		chain := h
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			chain = r.middlewares[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			if !commit.Committed() {
				r.dispatchError(reqCtx, err)
			}
		}
		return nil
	}
}

// dispatchError routes an error through the shared adapter error-dispatch
// helper, which uses the v2 response registry if configured and falls back
// to a sanitized 500 response.
func (r *FiberRouter) dispatchError(ctx *v2wf.RequestContext, err error) {
	response.DispatchError(r.respHandler, ctx, err)
}

// Ensure FiberRouter implements routing.Router.
var _ routing.Router = (*FiberRouter)(nil)

// Ensure FiberRouter implements routing.RouteGroup.
var _ routing.RouteGroup = (*FiberRouter)(nil)

// GetFiberCtx extracts the fiber.Ctx from a LegacyContext created by
// FiberRouter. Since v2 now passes *fiber.Ctx directly as LegacyContext,
// this helper type-asserts it.
func GetFiberCtx(ctx any) *fiber.Ctx {
	if c, ok := ctx.(*fiber.Ctx); ok {
		return c
	}
	return nil
}
