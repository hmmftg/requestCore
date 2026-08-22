package libGin

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// GinRouter implements routing.Router and routing.RouteGroup for Gin.
type GinRouter struct {
	engine       *gin.Engine
	group        *gin.RouterGroup
	middlewares  []routing.Middleware
	notFound     routing.Handler
	methodNA     routing.Handler
	legacyParser func(any) GinParserV2
}

// NewRouter creates a new GinRouter from a Gin engine.
func NewRouter(engine *gin.Engine) *GinRouter {
	return &GinRouter{
		engine:       engine,
		group:        &engine.RouterGroup,
		legacyParser: InitContextV2,
	}
}

// NewRouterFromGroup creates a new GinRouter from an existing Gin RouterGroup.
func NewRouterFromGroup(group *gin.RouterGroup) *GinRouter {
	return &GinRouter{
		engine:       nil,
		group:        group,
		legacyParser: InitContextV2,
	}
}

// Native returns the underlying Gin engine or router group.
func (r *GinRouter) Native() any {
	if r.engine != nil {
		return r.engine
	}
	return r.group
}

// Group creates a sub-group with the given prefix.
func (r *GinRouter) Group(prefix string) routing.RouteGroup {
	return &GinRouter{
		engine:       r.engine,
		group:        r.group.Group(prefix),
		middlewares:  r.middlewares,
		legacyParser: r.legacyParser,
	}
}

// With returns a new group with additional middleware.
func (r *GinRouter) With(middleware ...routing.Middleware) routing.RouteGroup {
	mws := make([]routing.Middleware, 0, len(r.middlewares)+len(middleware))
	mws = append(mws, r.middlewares...)
	mws = append(mws, middleware...)
	return &GinRouter{
		engine:       r.engine,
		group:        r.group,
		middlewares:  mws,
		legacyParser: r.legacyParser,
	}
}

// Handle registers a handler for the given method and pattern.
func (r *GinRouter) Handle(method, pattern string, handler routing.Handler) error {
	if err := routing.ValidatePattern(pattern); err != nil {
		return err
	}

	ginPattern := routing.TranslatePattern(pattern, "gin")
	wrapped := r.wrapHandler(handler)

	r.group.Handle(method, ginPattern, wrapped)
	return nil
}

// Get registers a GET handler.
func (r *GinRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

// Post registers a POST handler.
func (r *GinRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

// Put registers a PUT handler.
func (r *GinRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

// Patch registers a PATCH handler.
func (r *GinRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

// Delete registers a DELETE handler.
func (r *GinRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

// Head registers a HEAD handler.
func (r *GinRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *GinRouter) NotFound(handler routing.Handler) {
	r.notFound = handler
	r.engine.NoRoute(r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *GinRouter) MethodNotAllowed(handler routing.Handler) {
	r.methodNA = handler
	r.engine.NoMethod(r.wrapHandler(handler))
}

// wrapHandler converts a v2 routing.Handler to a gin.HandlerFunc,
// running the middleware chain and building the RequestContext.
func (r *GinRouter) wrapHandler(h routing.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		parser := r.legacyParser(c)
		reqCtx := &v2wf.RequestContext{
			Context:       c.Request.Context(),
			LegacyContext: c,
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
			// If the handler returns an error and no response was written,
			// the error should be handled by the error handler registry.
			// The caller is responsible for setting up the registry.
			// For now, we abort with 500 if no response was written.
			if !responseWritten(parser) {
				_ = parser.SendResponse(500, "application/json", []byte(`{"errors":[{"code":"INTERNAL","description":"Internal server error"}]}`))
			}
			c.Abort()
		}
	}
}

// responseWritten checks if the parser has already written a response.
func responseWritten(parser GinParserV2) bool {
	return parser.Ctx.Writer.Written()
}

// Ensure GinRouter implements routing.Router.
var _ routing.Router = (*GinRouter)(nil)

// Ensure GinRouter implements routing.RouteGroup.
var _ routing.RouteGroup = (*GinRouter)(nil)

// Suppress unused import warning for context.
var _ = context.Background

// Suppress unused import warning for fmt.
var _ = fmt.Sprintf
