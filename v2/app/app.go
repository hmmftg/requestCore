// Package app provides a framework-neutral application bootstrap for
// v2 requestCore applications.
//
// The App composes the v2 Runtime with framework-specific routers,
// middleware, and lifecycle management. It supports Gin, Fiber, and
// net/http+chi as underlying frameworks.
package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
	"github.com/hmmftg/requestCore/v2/workers"
)

// Framework identifies the underlying web framework.
type Framework string

const (
	FrameworkGin     Framework = "gin"
	FrameworkFiber   Framework = "fiber"
	FrameworkChi     Framework = "chi"
	FrameworkNetHTTP Framework = "net/http"
)

// Config holds the configuration for bootstrapping a v2 application.
type Config struct {
	// Framework specifies the underlying web framework.
	Framework Framework

	// Renderer is the default response renderer.
	// Default: JSONRenderer.
	Renderer renderers.Renderer

	// WorkerConfig configures the in-process worker pool.
	// Default: DefaultConfig().
	WorkerConfig workers.Config

	// SessionStore is the session store for session middleware.
	// Default: NoOpStore.
	SessionStore session.Store

	// SessionSecret is the secret used for signing session cookies.
	// Required if SessionStore is not NoOpStore.
	SessionSecret string

	// LegacyCore is the v1 RequestCoreInterface for infrastructure access.
	// May be nil for pure v2 applications.
	LegacyCore requestCore.RequestCoreInterface

	// LegacyHandler is the v1 WebHanlder for error response formatting.
	// Default: empty WebHanlder.
	LegacyHandler response.WebHanlder

	// Middlewares are global middleware applied to all routes.
	Middlewares []routing.Middleware

	// NotFound is the handler for unmatched routes.
	// If nil, a default 404 JSON response is used.
	NotFound routing.Handler

	// MethodNotAllowed is the handler for disallowed methods.
	// If nil, a default 405 JSON response is used.
	MethodNotAllowed routing.Handler
}

// App is the v2 application instance. It composes the router,
// response handler, worker pool, and session manager.
type App struct {
	Router      routing.Router
	RespHandler *v2response.Handler
	Registry    v2response.Registry
	Renderer    renderers.Renderer
	Worker      workers.Worker
	Sessions    *session.Manager
	Middlewares []routing.Middleware

	// nativeServer holds the underlying framework server (e.g. *http.Server,
	// *fiber.App). Set by Start to enable graceful shutdown.
	// Protected by serverMu to prevent data races between Start and Shutdown.
	serverMu     sync.Mutex
	nativeServer any
}

// Bootstrap creates a new v2 App with the given configuration.
// It initializes the router, response handler, worker pool, and session
// manager based on the specified framework.
//
// The returned App is ready for route registration via Register()
// or direct router access via Router.
func Bootstrap(config Config) (*App, error) {
	// Validate configuration before allocating any resources.
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	// Set defaults
	if config.Renderer == nil {
		config.Renderer = renderers.JSONRenderer{}
	}
	if config.SessionStore == nil {
		config.SessionStore = session.NoOpStore{}
	}

	// Create response handler
	registry := v2response.NewRegistry(nil)
	registry.SetFallback(v2response.LegacyFallback(config.LegacyHandler))
	respHandler := v2response.NewHandler(registry, config.Renderer, config.LegacyHandler)

	// Create worker pool
	worker := workers.NewInProcessWorker(config.WorkerConfig)

	// Create session manager
	sessionMgr := session.NewManager(config.SessionStore)

	// Create router based on framework
	router, err := createRouter(config.Framework)
	if err != nil {
		return nil, fmt.Errorf("app: failed to create router: %w", err)
	}

	// Wire the error handler into the router so that handler errors
	// are routed through the v2 error registry.
	wireErrorHandler(router, respHandler)

	// Apply global middleware by wrapping the router's route group.
	// Global middleware is applied at the root group level so all
	// routes inherit it.
	if len(config.Middlewares) > 0 {
		router = &middlewareRouter{router: router, middlewares: config.Middlewares}
	}

	// Set not found / method not allowed handlers. Defaults provide
	// JSON error responses consistent with the v2 response format.
	if config.NotFound != nil {
		router.NotFound(config.NotFound)
	} else {
		router.NotFound(defaultNotFound(respHandler))
	}
	if config.MethodNotAllowed != nil {
		router.MethodNotAllowed(config.MethodNotAllowed)
	} else {
		router.MethodNotAllowed(defaultMethodNotAllowed(respHandler))
	}

	return &App{
		Router:      router,
		RespHandler: respHandler,
		Registry:    registry,
		Renderer:    config.Renderer,
		Worker:      worker,
		Sessions:    sessionMgr,
		Middlewares: config.Middlewares,
	}, nil
}

// wireErrorHandler installs the v2 response handler on routers that
// support SetErrorHandler (all v2 adapter routers).
func wireErrorHandler(router routing.Router, handler *v2response.Handler) {
	type errorHandlerSetter interface {
		SetErrorHandler(*v2response.Handler)
	}
	if s, ok := router.(errorHandlerSetter); ok {
		s.SetErrorHandler(handler)
	}
}

// validateConfig checks the Bootstrap configuration before any resources
// are allocated. It returns an error describing the first issue found.
func validateConfig(config Config) error {
	// Framework must be one of the supported values.
	switch config.Framework {
	case FrameworkGin, FrameworkFiber, FrameworkChi, FrameworkNetHTTP:
	default:
		return fmt.Errorf("app: unsupported framework: %q", config.Framework)
	}

	// WorkerConfig must have positive worker count if non-zero
	// (zero is allowed and defaults to NumCPU in NewInProcessWorker).
	if config.WorkerConfig.WorkerCount < 0 {
		return fmt.Errorf("app: WorkerConfig.WorkerCount must be >= 0, got %d", config.WorkerConfig.WorkerCount)
	}
	if config.WorkerConfig.QueueSize < 0 {
		return fmt.Errorf("app: WorkerConfig.QueueSize must be >= 0, got %d", config.WorkerConfig.QueueSize)
	}

	// SessionSecret is documented as required when SessionStore is not
	// NoOpStore. If a non-NoOp store is provided without a secret, the
	// CookieStore cannot sign tokens. We warn by returning an error since
	// this is a configuration mistake that would cause runtime failures.
	if config.SessionSecret != "" && config.SessionStore == nil {
		// Secret provided but no store — this is fine, NoOpStore will be
		// used and the secret is ignored. Not an error.
	}
	if config.SessionStore != nil {
		if _, isNoOp := config.SessionStore.(session.NoOpStore); !isNoOp && config.SessionSecret == "" {
			return fmt.Errorf("app: SessionSecret is required when SessionStore is not NoOpStore")
		}
	}

	return nil
}

// defaultNotFound returns a 404 handler that emits a JSON error response
// through the v2 response handler.
func defaultNotFound(h *v2response.Handler) routing.Handler {
	return func(ctx *v2wf.RequestContext) error {
		return h.OKWithStatus(ctx, http.StatusNotFound, map[string]any{
			"error":   "not_found",
			"message": "The requested resource was not found",
		})
	}
}

// defaultMethodNotAllowed returns a 405 handler that emits a JSON error
// response through the v2 response handler.
func defaultMethodNotAllowed(h *v2response.Handler) routing.Handler {
	return func(ctx *v2wf.RequestContext) error {
		return h.OKWithStatus(ctx, http.StatusMethodNotAllowed, map[string]any{
			"error":   "method_not_allowed",
			"message": "The HTTP method is not allowed for this resource",
		})
	}
}

// createRouter creates a router for the specified framework.
func createRouter(framework Framework) (routing.Router, error) {
	switch framework {
	case FrameworkGin:
		return createGinRouter()
	case FrameworkFiber:
		return createFiberRouter()
	case FrameworkChi:
		return createChiRouter()
	case FrameworkNetHTTP:
		return createNetHTTPRouter()
	default:
		return nil, fmt.Errorf("app: unsupported framework: %s", framework)
	}
}

// middlewareRouter wraps a Router so that all route groups inherit
// the configured global middleware. This ensures that middleware
// registered at bootstrap time is applied to every route.
type middlewareRouter struct {
	router      routing.Router
	middlewares []routing.Middleware
}

func (m *middlewareRouter) Native() any {
	return m.router.Native()
}

func (m *middlewareRouter) Group(prefix string) routing.RouteGroup {
	return m.router.Group(prefix).With(m.middlewares...)
}

func (m *middlewareRouter) With(middlewares ...routing.Middleware) routing.RouteGroup {
	all := make([]routing.Middleware, 0, len(m.middlewares)+len(middlewares))
	all = append(all, m.middlewares...)
	all = append(all, middlewares...)
	return m.router.With(all...)
}

func (m *middlewareRouter) Handle(method, pattern string, handler routing.Handler) error {
	return m.router.Handle(method, pattern, m.wrap(handler))
}

func (m *middlewareRouter) Get(pattern string, handler routing.Handler) error {
	return m.router.Get(pattern, m.wrap(handler))
}

func (m *middlewareRouter) Post(pattern string, handler routing.Handler) error {
	return m.router.Post(pattern, m.wrap(handler))
}

func (m *middlewareRouter) Put(pattern string, handler routing.Handler) error {
	return m.router.Put(pattern, m.wrap(handler))
}

func (m *middlewareRouter) Patch(pattern string, handler routing.Handler) error {
	return m.router.Patch(pattern, m.wrap(handler))
}

func (m *middlewareRouter) Delete(pattern string, handler routing.Handler) error {
	return m.router.Delete(pattern, m.wrap(handler))
}

func (m *middlewareRouter) Head(pattern string, handler routing.Handler) error {
	return m.router.Head(pattern, m.wrap(handler))
}

// wrap applies the global middleware chain to the given handler so that
// direct route registration (Handle/Get/Post/...) inherits bootstrap
// middleware, matching the behavior of Group and With.
func (m *middlewareRouter) wrap(h routing.Handler) routing.Handler {
	chain := h
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		chain = m.middlewares[i](chain)
	}
	return chain
}

func (m *middlewareRouter) NotFound(handler routing.Handler) {
	m.router.NotFound(handler)
}

func (m *middlewareRouter) MethodNotAllowed(handler routing.Handler) {
	m.router.MethodNotAllowed(handler)
}

// Ensure middlewareRouter implements routing.Router.
var _ routing.Router = (*middlewareRouter)(nil)

// Register registers a route group with middleware.
func (a *App) Register(prefix string, middlewares ...routing.Middleware) routing.RouteGroup {
	group := a.Router.Group(prefix)
	if len(middlewares) > 0 {
		group = group.With(middlewares...)
	}
	return group
}

// Start starts the HTTP server on the given address.
// For Gin and chi/net/http, this uses http.Server.
// For Fiber, this uses fiber.App.Listen.
func (a *App) Start(addr string) error {
	return a.StartWithContext(context.Background(), addr)
}

// StartWithContext starts the HTTP server with the given context.
// When the context is cancelled, the server shuts down gracefully.
//
// The error handler registry is frozen before the server starts so
// that route handlers cannot modify error handlers at runtime.
func (a *App) StartWithContext(ctx context.Context, addr string) error {
	// Freeze the registry so no new error handlers can be registered
	// after startup. This prevents accidental mutation during serving.
	if a.Registry != nil {
		a.Registry.Freeze()
	}

	native := a.Router.Native()

	switch server := native.(type) {
	case *http.Server:
		server.Addr = addr
		a.serverMu.Lock()
		a.nativeServer = server
		a.serverMu.Unlock()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
		}()
		return server.ListenAndServe()
	case http.Handler:
		httpServer := &http.Server{
			Addr:    addr,
			Handler: server,
		}
		a.serverMu.Lock()
		a.nativeServer = httpServer
		a.serverMu.Unlock()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
		}()
		return httpServer.ListenAndServe()
	default:
		// Fiber or other framework
		a.serverMu.Lock()
		a.nativeServer = native
		a.serverMu.Unlock()
		return startFiber(native, addr)
	}
}

// Shutdown gracefully shuts down the application: it stops the HTTP
// server, then shuts down the worker pool. Both operations are bounded
// by the given context.
//
// Shutdown is safe to call concurrently with StartWithContext. If the
// server has not yet been registered (StartWithContext hasn't stored it
// yet), Shutdown waits briefly for it to appear, then proceeds to shut
// down the worker pool.
func (a *App) Shutdown(ctx context.Context) error {
	// Wait for the server to be registered if StartWithContext is
	// concurrently in progress. This closes the race window where
	// Shutdown sees nil nativeServer because Start hasn't stored it yet.
	deadline, hasDeadline := ctx.Deadline()
	for i := 0; i < 50; i++ { // up to ~500ms
		a.serverMu.Lock()
		server := a.nativeServer
		a.serverMu.Unlock()
		if server != nil {
			break
		}
		if hasDeadline && time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	a.serverMu.Lock()
	server := a.nativeServer
	a.serverMu.Unlock()

	var httpErr error
	if server != nil {
		if httpServer, ok := server.(*http.Server); ok {
			httpErr = httpServer.Shutdown(ctx)
		} else {
			// Fiber shutdown
			httpErr = shutdownFiber(server, ctx)
		}
	}

	// Shut down the worker pool after the HTTP server stops accepting
	// new requests, so in-flight jobs can complete within the context.
	workerErr := a.Worker.Shutdown(ctx)

	// Return the first non-nil error, preferring the HTTP error.
	if httpErr != nil {
		return httpErr
	}
	return workerErr
}

// Close stops the worker pool and releases resources.
// It uses a 10-second timeout for the worker shutdown.
//
// For full graceful shutdown including the HTTP server, use Shutdown
// instead. Close is primarily useful in tests where the HTTP server
// was never started.
func (a *App) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.Worker.Shutdown(ctx)
}
