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

// NotFound sets the handler for unmatched routes.
func (r *FiberRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
	r.app.Use("*", r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
// Fiber v2 does not have a built-in 405 handler, so we register a
// catch-all that checks the method against registered routes and
// dispatches the 405 handler when the path matches but the method doesn't.
func (r *FiberRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
	// Register a catch-all that fires after all specific routes.
	// It checks if the path matches a registered route with a different
	// method and dispatches the 405 handler if so.
	r.app.Use("*", func(c *fiber.Ctx) error {
		// If a response was already sent by a matched route, skip.
		if c.Response().StatusCode() != 404 {
			return nil
		}
		// Check if the path matches any registered route pattern.
		path := c.Path()
		method := c.Method()
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
		if !pathMatches {
			return nil
		}
		// Dispatch the 405 handler through the v2 pipeline.
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
