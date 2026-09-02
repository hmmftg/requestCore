// Package app provides a framework-neutral application bootstrap for
// v2 requestCore applications.
//
// The App composes the v2 Runtime with framework-specific routers,
// middleware, and lifecycle management. It supports Gin, Fiber, and
// net/http+chi as underlying frameworks.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/hmmftg/requestCore/v2/endpoint"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
	"github.com/hmmftg/requestCore/v2/telemetry"
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

	// Renderer is the default response renderer. It is also used as
	// the executor's default encoder.
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

	// ProblemMapper is the error-to-Problem mapper registry used by
	// the routing error handler and the executor.
	// If nil, response.DefaultMapperRegistry() is used.
	ProblemMapper *response.MapperRegistry

	// OperationRegistry is the operation metadata registry used by
	// the executor. If nil, operation.NewRegistry() is used.
	OperationRegistry operation.Registry

	// Executor is the canonical endpoint executor. If nil, one is
	// created with the above defaults (and the Renderer as encoder).
	Executor *endpoint.Executor

	// Middlewares are global middleware applied to all routes.
	Middlewares []routing.Middleware

	// NotFound is the handler for unmatched routes.
	// If nil, a default 404 RFC 9457 Problem response is used.
	NotFound routing.Handler

	// MethodNotAllowed is the handler for disallowed methods.
	// If nil, a default 405 RFC 9457 Problem response is used.
	MethodNotAllowed routing.Handler

	// Logger is the base logger for worker and scheduler jobs.
	// If nil, slog.Default() is used.
	Logger *slog.Logger

	// TelemetrySink is the telemetry sink for worker and scheduler
	// lifecycle events. If nil, telemetry.NewSlogSink(Logger) is used.
	TelemetrySink telemetry.Sink
}

// App is the v2 application instance. It composes the router,
// endpoint executor, problem mapper, operation registry, worker pool,
// scheduler, and session manager.
type App struct {
	Router            routing.Router
	Renderer          renderers.Renderer
	Executor          *endpoint.Executor
	ProblemMapper     *response.MapperRegistry
	OperationRegistry operation.Registry
	Worker            workers.Worker
	Scheduler         *workers.Scheduler
	Sessions          *session.Manager
	Middlewares       []routing.Middleware

	// nativeServer holds the underlying framework server (e.g. *http.Server,
	// *fiber.App). Set by Start to enable graceful shutdown.
	// Protected by serverMu to prevent data races between Start and Shutdown.
	serverMu     sync.Mutex
	nativeServer any

	// serverRegistered is closed when StartWithContext stores nativeServer,
	// allowing Shutdown to wait deterministically instead of polling.
	serverRegistered chan struct{}
	// serverRegOnce ensures serverRegistered is closed at most once.
	serverRegOnce sync.Once
}

// Bootstrap creates a new v2 App with the given configuration.
// It initializes the router, executor, worker pool, and session
// manager based on the specified framework.
//
// The returned App is ready for route registration via Register()
// or direct router access via Router. Operation and mapper registries
// are frozen at StartWithContext, not here, so routes can still be
// registered between Bootstrap and Start.
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

	// Default the problem mapper registry.
	if config.ProblemMapper == nil {
		config.ProblemMapper = response.DefaultMapperRegistry()
	}

	// Default the operation registry.
	if config.OperationRegistry == nil {
		config.OperationRegistry = operation.NewRegistry()
	}

	// Default the telemetry sink to an observable SlogSink.
	if config.TelemetrySink == nil {
		config.TelemetrySink = telemetry.NewSlogSink(config.Logger)
	}

	// Use or create the canonical endpoint executor. The executor
	// runs the canonical lifecycle (bind → validate → execute →
	// encode → commit → observe) and provides telemetry via its
	// configured telemetry.Sink.
	executor := config.Executor
	if executor == nil {
		executor = endpoint.NewExecutor(
			endpoint.WithRegistry(config.OperationRegistry),
			endpoint.WithProblemMapper(config.ProblemMapper),
			endpoint.WithTelemetrySink(config.TelemetrySink),
			endpoint.WithExecutorEncoder(config.Renderer),
		)
	}

	// Determine the base logger for workers and scheduler.
	workerLogger := config.Logger
	if workerLogger == nil {
		workerLogger = slog.Default()
	}

	// Create worker pool with telemetry observability.
	workerConfig := config.WorkerConfig
	workerConfig.Logger = workerLogger
	workerConfig.Sink = config.TelemetrySink
	worker := workers.NewInProcessWorker(workerConfig)

	// Create scheduler for periodic background tasks with telemetry.
	scheduler := workers.NewScheduler(workers.SchedulerConfig{
		Logger: workerLogger,
		Sink:   config.TelemetrySink,
	})

	// Create session manager
	sessionMgr := session.NewManager(config.SessionStore)

	// Create router based on framework
	router, err := createRouter(config.Framework)
	if err != nil {
		return nil, fmt.Errorf("app: failed to create router: %w", err)
	}

	// Wire the mapper-backed error handler into the router so that
	// handler errors are mapped to RFC 9457 Problems and written
	// through the transport.
	wireErrorHandler(router, makeErrorHandler(config.ProblemMapper))

	// Apply global middleware by wrapping the router's route group.
	// Global middleware is applied at the root group level so all
	// routes inherit it.
	if len(config.Middlewares) > 0 {
		router = &middlewareRouter{router: router, middlewares: config.Middlewares}
	}

	// Set not found / method not allowed handlers. Defaults write
	// RFC 9457 Problem responses.
	if config.NotFound != nil {
		router.NotFound(config.NotFound)
	} else {
		router.NotFound(defaultNotFound())
	}
	if config.MethodNotAllowed != nil {
		router.MethodNotAllowed(config.MethodNotAllowed)
	} else {
		router.MethodNotAllowed(defaultMethodNotAllowed())
	}

	return &App{
		Router:            router,
		Renderer:          config.Renderer,
		Executor:          executor,
		ProblemMapper:     config.ProblemMapper,
		OperationRegistry: config.OperationRegistry,
		Worker:            worker,
		Scheduler:         scheduler,
		Sessions:          sessionMgr,
		Middlewares:       config.Middlewares,
		serverRegistered:  make(chan struct{}),
	}, nil
}

// wireErrorHandler installs the error handler on the router.
func wireErrorHandler(router routing.Router, handler routing.ErrorHandler) {
	router.SetErrorHandler(handler)
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

// defaultNotFound returns a 404 handler that writes an RFC 9457
// Problem response.
func defaultNotFound() routing.Handler {
	return func(ctx *request.Context, transport routing.Transport) error {
		problem := response.NewProblemWithCode(http.StatusNotFound, "Not Found", "NOT_FOUND")
		return response.WriteProblem(ctx, transport, problem)
	}
}

// defaultMethodNotAllowed returns a 405 handler that writes an RFC 9457
// Problem response.
func defaultMethodNotAllowed() routing.Handler {
	return func(ctx *request.Context, transport routing.Transport) error {
		problem := response.NewProblemWithCode(http.StatusMethodNotAllowed, "Method Not Allowed", "METHOD_NOT_ALLOWED")
		return response.WriteProblem(ctx, transport, problem)
	}
}

// makeErrorHandler creates a routing.ErrorHandler that maps errors to
// RFC 9457 Problems via the given MapperRegistry and writes them
// through the transport.
func makeErrorHandler(mapper *response.MapperRegistry) routing.ErrorHandler {
	return func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, mapper, err)
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
	if len(m.middlewares) == 0 {
		return h
	}
	return routing.Chain(m.middlewares...)(h)
}

func (m *middlewareRouter) NotFound(handler routing.Handler) {
	m.router.NotFound(handler)
}

func (m *middlewareRouter) MethodNotAllowed(handler routing.Handler) {
	m.router.MethodNotAllowed(handler)
}

func (m *middlewareRouter) SetErrorHandler(handler routing.ErrorHandler) {
	m.router.SetErrorHandler(handler)
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
// The operation registry and problem mapper are frozen before the
// server starts so that routes and error mappings cannot be modified
// at runtime. Freezing happens here (not at Bootstrap) so routes can
// still be registered between Bootstrap and Start.
func (a *App) StartWithContext(ctx context.Context, addr string) error {
	// Freeze the operation registry and problem mapper so no new
	// entries can be registered after startup. This prevents
	// accidental mutation during serving.
	if a.OperationRegistry != nil {
		a.OperationRegistry.Freeze()
	}
	if a.ProblemMapper != nil {
		a.ProblemMapper.Freeze()
	}

	// Start the scheduler so registered periodic jobs begin ticking.
	if a.Scheduler != nil {
		a.Scheduler.Start()
	}

	native := a.Router.Native()

	switch server := native.(type) {
	case *http.Server:
		server.Addr = addr
		a.serverMu.Lock()
		a.nativeServer = server
		a.serverMu.Unlock()
		a.serverRegOnce.Do(func() { close(a.serverRegistered) })
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
		a.serverRegOnce.Do(func() { close(a.serverRegistered) })
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
		a.serverRegOnce.Do(func() { close(a.serverRegistered) })
		return startFiber(native, addr)
	}
}

// Shutdown gracefully shuts down the application: it stops the HTTP
// server, then shuts down the worker pool. Both operations are bounded
// by the given context.
//
// Shutdown is safe to call concurrently with StartWithContext. It waits
// deterministically for StartWithContext to register the server (via a
// closed channel) before proceeding, eliminating the prior polling race.
//
// http.ErrServerClosed is treated as a clean termination (return nil)
// because it indicates the server was already shut down, typically by
// a concurrent StartWithContext context cancellation.
func (a *App) Shutdown(ctx context.Context) error {
	// Wait deterministically for StartWithContext to register the server.
	// This replaces the previous polling loop and closes the race window
	// where Shutdown sees nil nativeServer because Start hasn't stored it yet.
	if a.serverRegistered != nil {
		select {
		case <-a.serverRegistered:
		case <-ctx.Done():
		}
	}

	a.serverMu.Lock()
	server := a.nativeServer
	a.serverMu.Unlock()

	var httpErr error
	if server != nil {
		if httpServer, ok := server.(*http.Server); ok {
			httpErr = httpServer.Shutdown(ctx)
			// http.ErrServerClosed is expected when StartWithContext's
			// context-driven shutdown races with an explicit Shutdown call.
			if errors.Is(httpErr, http.ErrServerClosed) {
				httpErr = nil
			}
		} else {
			// Fiber shutdown
			httpErr = shutdownFiber(server, ctx)
		}
	}

	// Shut down the worker pool after the HTTP server stops accepting
	// new requests, so in-flight jobs can complete within the context.
	workerErr := a.Worker.Shutdown(ctx)

	// Shut down the scheduler after the worker pool, so periodic
	// tasks stop ticking and in-flight ticks complete.
	var schedulerErr error
	if a.Scheduler != nil {
		schedulerErr = a.Scheduler.Shutdown(ctx)
	}

	// Return the first non-nil error, preferring the HTTP error.
	if httpErr != nil {
		return httpErr
	}
	if workerErr != nil {
		return workerErr
	}
	return schedulerErr
}

// Close stops the worker pool and scheduler, and releases resources.
// It uses a 10-second timeout for the shutdown.
//
// For full graceful shutdown including the HTTP server, use Shutdown
// instead. Close is primarily useful in tests where the HTTP server
// was never started.
func (a *App) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	workerErr := a.Worker.Shutdown(ctx)
	var schedulerErr error
	if a.Scheduler != nil {
		schedulerErr = a.Scheduler.Shutdown(ctx)
	}
	if workerErr != nil {
		return workerErr
	}
	return schedulerErr
}
