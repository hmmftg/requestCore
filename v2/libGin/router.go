package libGin

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	legacy "github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/response"
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
	legacyParser func(any) *GinParserV2
	registry     response.Registry
	respHandler  *response.Handler
}

// NewRouter creates a new GinRouter from a Gin engine.
func NewRouter(engine *gin.Engine) *GinRouter {
	r := &GinRouter{
		engine:       engine,
		group:        &engine.RouterGroup,
		legacyParser: InitContextV2,
	}
	return r
}

// NewRouterFromGroup creates a new GinRouter from an existing Gin RouterGroup.
func NewRouterFromGroup(group *gin.RouterGroup) *GinRouter {
	return &GinRouter{
		engine:       nil,
		group:        group,
		legacyParser: InitContextV2,
	}
}

// SetErrorHandler installs the v2 response handler and error registry used
// for centralized error dispatch. When set, handler errors are routed
// through the registry instead of emitting hard-coded 500 responses.
func (r *GinRouter) SetErrorHandler(handler *response.Handler) {
	if handler == nil {
		return
	}
	r.respHandler = handler
	r.registry = handler.Registry()
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
		registry:     r.registry,
		respHandler:  r.respHandler,
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
		registry:     r.registry,
		respHandler:  r.respHandler,
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

// NotFound sets the handler for unmatched routes. This requires an engine
// scope; calling it on a group-only router (created via NewRouterFromGroup)
// panics because 404 registration is engine-wide.
func (r *GinRouter) NotFound(handler routing.Handler) {
	if r.engine == nil {
		panic("libGin: NotFound requires an engine-scoped router; use NewRouter instead of NewRouterFromGroup")
	}
	r.notFound = handler
	r.engine.NoRoute(r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods. This requires
// an engine scope; calling it on a group-only router panics.
func (r *GinRouter) MethodNotAllowed(handler routing.Handler) {
	if r.engine == nil {
		panic("libGin: MethodNotAllowed requires an engine-scoped router; use NewRouter instead of NewRouterFromGroup")
	}
	r.methodNA = handler
	engine := r.engine
	engine.HandleMethodNotAllowed = true
	engine.NoMethod(r.wrapHandler(handler))
}

// wrapHandler converts a v2 routing.Handler to a gin.HandlerFunc,
// running the middleware chain and building the RequestContext.
func (r *GinRouter) wrapHandler(h routing.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		parser := r.legacyParser(c)
		commit := &v2wf.CommitState{}
		parser.SetCommitState(commit)
		reqCtx := &v2wf.RequestContext{
			// LegacyContext is the native *gin.Context expected by
			// libContext.InitContext.
			LegacyContext: c,
			Parser:        parser,
			Legacy: legacy.WebFramework{
				Parser: parser,
			},
		}
		reqCtx.SetCommitState(commit)
		// Use the request context for cancellation/tracing.
		reqCtx.Context = c.Request.Context()

		// Apply middleware chain
		chain := h
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			chain = r.middlewares[i](chain)
		}

		if err := chain(reqCtx); err != nil {
			// If the handler returns an error and no response was
			// committed, route it through the error registry.
			if !commit.Committed() {
				r.dispatchError(reqCtx, err)
			}
			c.Abort()
		}
	}
}

// dispatchError routes an error through the shared adapter error-dispatch
// helper, which uses the v2 response registry if configured and falls back
// to a sanitized 500 response.
func (r *GinRouter) dispatchError(ctx *v2wf.RequestContext, err error) {
	response.DispatchError(r.respHandler, ctx, err)
}

// Ensure GinRouter implements routing.Router.
var _ routing.Router = (*GinRouter)(nil)

// Ensure GinRouter implements routing.RouteGroup.
var _ routing.RouteGroup = (*GinRouter)(nil)

// Suppress unused import warning for context.
var _ = context.Background

// Suppress unused import warning for fmt.
var _ = fmt.Sprintf
