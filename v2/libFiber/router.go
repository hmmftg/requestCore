package libFiber

import (
	"context"

	"github.com/gofiber/fiber/v2"

	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// FiberRouter implements routing.Router and routing.RouteGroup for Fiber.
type FiberRouter struct {
	app         *fiber.App
	group       fiber.Router
	middlewares []routing.Middleware
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
	r.app.Use("*", r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *FiberRouter) MethodNotAllowed(handler routing.Handler) {
	// Fiber does not have a built-in MethodNotAllowed handler.
	// This is a limitation; the handler is stored but not wired.
}

// wrapHandler converts a v2 routing.Handler to a fiber.Handler,
// running the middleware chain and building the RequestContext.
func (r *FiberRouter) wrapHandler(h routing.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		parser := InitContextV2(c)
		reqCtx := &v2wf.RequestContext{
			Context:       c.UserContext(),
			LegacyContext: context.WithValue(context.Background(), fiberCtxKey{}, c),
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
			// If no response body was written, send a 500 error.
			if len(c.Response().Body()) == 0 {
				c.Status(500)
				return c.SendString(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`)
			}
			return err
		}
		return nil
	}
}

// Ensure FiberRouter implements routing.Router.
var _ routing.Router = (*FiberRouter)(nil)

// Ensure FiberRouter implements routing.RouteGroup.
var _ routing.RouteGroup = (*FiberRouter)(nil)

// fiberCtxKey is used to store the fiber.Ctx in the LegacyContext.
type fiberCtxKey struct{}

// GetFiberCtx extracts the fiber.Ctx from a LegacyContext created by FiberRouter.
func GetFiberCtx(ctx context.Context) *fiber.Ctx {
	if v, ok := ctx.Value(fiberCtxKey{}).(*fiber.Ctx); ok {
		return v
	}
	return nil
}
