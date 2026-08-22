// Package response provides the v2 response handler with a centralized
// error handler registry and pluggable renderers.
//
// The error handler registry allows applications to register per-status-code
// handlers for customized error responses. The v2 response handler delegates
// to the v1 [github.com/hmmftg/requestCore/response.WebHanlder] for the
// actual Splunk transaction pipeline and localization, adding only the
// registry layer on top.
package response

import (
	"errors"
	"net/http"
	"sync"

	legacyError "github.com/hmmftg/requestCore/libError"
	legacyResponse "github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/webFramework"
)

// Registry is the error handler registry interface.
// It is mutable during startup and frozen before serving.
type Registry interface {
	// Register associates a handler with an HTTP status code.
	// Returns an error if the status code is invalid or the registry is frozen.
	Register(status int, handler webFramework.ErrorHandler) error

	// SetFallback sets the fallback handler invoked when no handler is
	// registered for a given status code. The default fallback delegates
	// to the v1 WebHanlder.Error method.
	SetFallback(handler webFramework.ErrorHandler) error

	// Resolve determines the HTTP status code for an error.
	Resolve(err error) int

	// Handle invokes the appropriate error handler for the given error.
	// It resolves the status, looks up a registered handler (or fallback),
	// and invokes it. If the handler fails to write a response, the
	// fallback is invoked exactly once.
	Handle(req *webFramework.RequestContext, err error) error

	// Freeze prevents further registration. Called after startup.
	Freeze()
}

// DefaultRegistry is the default Registry implementation.
type DefaultRegistry struct {
	mu       sync.RWMutex
	handlers map[int]webFramework.ErrorHandler
	fallback webFramework.ErrorHandler
	resolver webFramework.StatusResolver
	frozen   bool
}

// NewRegistry creates a new DefaultRegistry with the given status resolver.
// If resolver is nil, DefaultStatusResolver is used.
func NewRegistry(resolver webFramework.StatusResolver) *DefaultRegistry {
	if resolver == nil {
		resolver = DefaultStatusResolver
	}
	return &DefaultRegistry{
		handlers: make(map[int]webFramework.ErrorHandler),
		resolver: resolver,
	}
}

// Register associates a handler with an HTTP status code.
func (r *DefaultRegistry) Register(status int, handler webFramework.ErrorHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return errors.New("response: registry is frozen")
	}
	if handler == nil {
		return errors.New("response: handler cannot be nil")
	}
	if status < 100 || status > 599 {
		return errors.New("response: invalid HTTP status code")
	}
	r.handlers[status] = handler
	return nil
}

// SetFallback sets the fallback error handler.
func (r *DefaultRegistry) SetFallback(handler webFramework.ErrorHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return errors.New("response: registry is frozen")
	}
	if handler == nil {
		return errors.New("response: fallback handler cannot be nil")
	}
	r.fallback = handler
	return nil
}

// Resolve determines the HTTP status code for an error.
func (r *DefaultRegistry) Resolve(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return r.resolver(err)
}

// Handle invokes the appropriate error handler for the given error.
func (r *DefaultRegistry) Handle(req *webFramework.RequestContext, err error) error {
	if err == nil {
		return nil
	}

	status := r.Resolve(err)
	ctx := webFramework.ErrorContext{
		Request: req,
		Error:   err,
		Status:  status,
	}

	r.mu.RLock()
	handler, ok := r.handlers[status]
	fallback := r.fallback
	r.mu.RUnlock()

	if ok {
		if hErr := handler(ctx); hErr != nil {
			if fallback != nil {
				return fallback(ctx)
			}
			return hErr
		}
		return nil
	}

	if fallback != nil {
		return fallback(ctx)
	}

	return errors.New("response: no handler or fallback registered")
}

// Freeze prevents further registration.
func (r *DefaultRegistry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// DefaultStatusResolver resolves the HTTP status code from an error by
// inspecting known error types in order:
//  1. libError.ErrorData via Action().Status
//  2. response.ErrorState via GetStatus()
//  3. Any type implementing interface{ HTTPStatus() int }
//  4. Fallback to 500 Internal Server Error
func DefaultStatusResolver(err error) int {
	if err == nil {
		return http.StatusOK
	}

	var libErr legacyError.ErrorData
	if errors.As(err, &libErr) {
		return libErr.Action().Status.Int()
	}

	var state legacyResponse.ErrorState
	if errors.As(err, &state) {
		s := state.GetStatus()
		if s > 0 {
			return s
		}
	}

	type httpStatuser interface {
		HTTPStatus() int
	}
	var hs httpStatuser
	if errors.As(err, &hs) {
		s := hs.HTTPStatus()
		if s > 0 {
			return s
		}
	}

	return http.StatusInternalServerError
}
