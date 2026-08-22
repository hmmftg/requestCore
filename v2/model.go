package requestcore

import (
	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libParams"
	"github.com/hmmftg/requestCore/libQuery"
	"github.com/hmmftg/requestCore/libQuery/liborm"
	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/session"
	"github.com/hmmftg/requestCore/v2/workers"
)

// Runtime is the v2 runtime façade providing access to all v2 features
// while delegating to the v1 RequestCoreInterface for existing infrastructure.
type Runtime interface {
	// Legacy returns the v1 RequestCoreInterface for access to existing
	// query, persistence, response, logging, and tracing infrastructure.
	Legacy() requestCore.RequestCoreInterface

	// Responder returns the v2 response handler with renderer support
	// and error handler registry.
	Responder() *v2response.Handler

	// ErrorHandlers returns the error handler registry.
	ErrorHandlers() v2response.Registry

	// DefaultRenderer returns the default renderer (JSON by default).
	DefaultRenderer() renderers.Renderer

	// WorkerPool returns the in-process worker pool.
	WorkerPool() workers.Worker

	// SessionManager returns the session manager.
	SessionManager() *session.Manager

	// V1 accessors — delegate to the legacy core so Model can be used
	// where RequestCoreInterface is expected.

	// GetDB returns the query runner interface (delegates to v1).
	GetDB() libQuery.QueryRunnerInterface
	// ORM returns the ORM interface (delegates to v1).
	ORM() liborm.OrmInterface
	// RequestTools returns the request interface (delegates to v1).
	RequestTools() libRequest.RequestInterface
	// ResponderV1 returns the v1 response handler (delegates to v1).
	ResponderV1() response.ResponseHandler
	// Params returns the parameter interface (delegates to v1).
	Params() libParams.ParamInterface
}

// Model is the default Runtime implementation. It composes the v1
// RequestCoreInterface with v2 features.
type Model struct {
	LegacyCore      requestCore.RequestCoreInterface
	ResponseHandler *v2response.Handler
	Errors          v2response.Registry
	Renderer        renderers.Renderer
	Worker          workers.Worker
	Sessions        *session.Manager
}

// Legacy returns the v1 RequestCoreInterface.
func (m *Model) Legacy() requestCore.RequestCoreInterface {
	return m.LegacyCore
}

// Responder returns the v2 response handler.
func (m *Model) Responder() *v2response.Handler {
	return m.ResponseHandler
}

// ErrorHandlers returns the error handler registry.
func (m *Model) ErrorHandlers() v2response.Registry {
	return m.Errors
}

// DefaultRenderer returns the default renderer.
func (m *Model) DefaultRenderer() renderers.Renderer {
	return m.Renderer
}

// WorkerPool returns the worker pool.
func (m *Model) WorkerPool() workers.Worker {
	return m.Worker
}

// SessionManager returns the session manager.
func (m *Model) SessionManager() *session.Manager {
	return m.Sessions
}

// GetDB delegates to the v1 core.
func (m *Model) GetDB() libQuery.QueryRunnerInterface {
	return m.LegacyCore.GetDB()
}

// ORM delegates to the v1 core.
func (m *Model) ORM() liborm.OrmInterface {
	return m.LegacyCore.ORM()
}

// RequestTools delegates to the v1 core.
func (m *Model) RequestTools() libRequest.RequestInterface {
	return m.LegacyCore.RequestTools()
}

// ResponderV1 delegates to the v1 core.
func (m *Model) ResponderV1() response.ResponseHandler {
	return m.LegacyCore.Responder()
}

// Params delegates to the v1 core.
func (m *Model) Params() libParams.ParamInterface {
	return m.LegacyCore.Params()
}

// NewModel creates a v2 Model from the given v1 core and v2 feature components.
// If any v2 component is nil, safe defaults are used.
func NewModel(
	legacyCore requestCore.RequestCoreInterface,
	legacyHandler response.WebHanlder,
	renderer renderers.Renderer,
	worker workers.Worker,
	sessionStore session.Store,
) *Model {
	registry := v2response.NewRegistry(nil)
	registry.SetFallback(v2response.LegacyFallback(legacyHandler))

	if renderer == nil {
		renderer = renderers.JSONRenderer{}
	}

	respHandler := v2response.NewHandler(registry, renderer, legacyHandler)

	if worker == nil {
		worker = workers.NewInProcessWorker(workers.DefaultConfig())
	}

	if sessionStore == nil {
		sessionStore = session.NoOpStore{}
	}

	return &Model{
		LegacyCore:      legacyCore,
		ResponseHandler: respHandler,
		Errors:          registry,
		Renderer:        renderer,
		Worker:          worker,
		Sessions:        session.NewManager(sessionStore),
	}
}
