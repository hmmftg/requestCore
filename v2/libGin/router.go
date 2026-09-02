// Package libGin provides the v2 Gin framework adapter for requestCore.
// It constructs request.Context and routing.Transport from gin.Context
// and registers handlers with the Gin engine.
package libGin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

// ginTransport implements routing.Transport using a gin.Context.
type ginTransport struct {
	c         *gin.Context
	committed bool
}

func (t *ginTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	if t.committed {
		return nil
	}
	t.committed = true
	for k, vs := range headers {
		for _, v := range vs {
			t.c.Writer.Header().Add(k, v)
		}
	}
	if contentType != "" {
		t.c.Writer.Header().Set("Content-Type", contentType)
	}
	t.c.Status(status)
	_, err := t.c.Writer.Write(body)
	return err
}

func (t *ginTransport) Committed() bool {
	return t.committed || t.c.Writer.Written()
}

// GinRouter implements routing.Router and routing.RouteGroup for Gin.
type GinRouter struct {
	engine       *gin.Engine
	group        *gin.RouterGroup
	middlewares  []routing.Middleware
	notFound     routing.Handler
	methodNA     routing.Handler
	errorHandler routing.ErrorHandler
}

// NewRouter creates a new GinRouter from a Gin engine.
func NewRouter(engine *gin.Engine) *GinRouter {
	return &GinRouter{
		engine: engine,
		group:  &engine.RouterGroup,
	}
}

// NewRouterFromGroup creates a new GinRouter from an existing Gin RouterGroup.
func NewRouterFromGroup(group *gin.RouterGroup) *GinRouter {
	return &GinRouter{
		engine: nil,
		group:  group,
	}
}

// SetErrorHandler sets the error handler for centralized error dispatch.
func (r *GinRouter) SetErrorHandler(handler routing.ErrorHandler) {
	r.errorHandler = handler
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
		errorHandler: r.errorHandler,
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
		errorHandler: r.errorHandler,
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

func (r *GinRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}

func (r *GinRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}

func (r *GinRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}

func (r *GinRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}

func (r *GinRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}

func (r *GinRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

// NotFound sets the handler for unmatched routes.
func (r *GinRouter) NotFound(handler routing.Handler) {
	if r.engine == nil {
		panic("libGin: NotFound requires an engine-scoped router; use NewRouter instead of NewRouterFromGroup")
	}
	r.notFound = handler
	r.engine.NoRoute(r.wrapHandler(handler))
}

// MethodNotAllowed sets the handler for disallowed methods.
func (r *GinRouter) MethodNotAllowed(handler routing.Handler) {
	if r.engine == nil {
		panic("libGin: MethodNotAllowed requires an engine-scoped router; use NewRouter instead of NewRouterFromGroup")
	}
	r.methodNA = handler
	r.engine.HandleMethodNotAllowed = true
	r.engine.NoMethod(r.wrapHandler(handler))
}

// buildContext creates a *request.Context from a gin.Context.
func buildContext(c *gin.Context) *request.Context {
	pathParams := make(map[string]string)
	for _, p := range c.Params {
		pathParams[p.Key] = p.Value
	}
	opts := []request.Option{
		request.WithMethod(c.Request.Method),
		request.WithPath(c.Request.URL.Path),
		request.WithRoutePattern(c.FullPath()),
		request.WithHeader(c.Request.Header),
		request.WithQuery(c.Request.URL.Query()),
		request.WithRemoteAddr(c.Request.RemoteAddr),
		request.WithNative(c),
		request.WithBodySource(request.NewBodySource(c.Request.Body)),
	}
	if len(pathParams) > 0 {
		opts = append(opts, request.WithPathParams(pathParams))
	}
	if len(c.Request.Cookies()) > 0 {
		opts = append(opts, request.WithCookies(c.Request.Cookies()))
	}
	return request.NewContext(c.Request.Context(), opts...)
}

// wrapHandler converts a routing.Handler to a gin.HandlerFunc.
func (r *GinRouter) wrapHandler(h routing.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := buildContext(c)
		transport := &ginTransport{c: c}

		chain := h
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			chain = r.middlewares[i](chain)
		}

		if err := chain(ctx, transport); err != nil {
			if !transport.Committed() && r.errorHandler != nil {
				r.errorHandler(ctx, transport, err)
			}
			c.Abort()
		}
	}
}

// Ensure interface implementations.
var _ routing.Router = (*GinRouter)(nil)
var _ routing.RouteGroup = (*GinRouter)(nil)
var _ routing.Transport = (*ginTransport)(nil)
