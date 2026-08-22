package response

import (
	"errors"
	"log/slog"
	"net/http"

	legacyError "github.com/hmmftg/requestCore/libError"
	legacyResponse "github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/status"
	"github.com/hmmftg/requestCore/webFramework"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// Handler is the v2 response handler. It provides renderer-based response
// methods and delegates error handling to the error handler registry.
//
// Unlike the v1 ResponseHandler interface, the v2 Handler takes
// *webFramework.RequestContext (which carries both v1 and v2 framework
// objects). It does not implement the v1 ResponseHandler interface
// because the context types differ. Use LegacyHandler() to obtain the
// underlying v1 WebHanlder for code that expects the v1 interface.
type Handler struct {
	registry      Registry
	defaultRend   renderers.Renderer
	legacyHandler legacyResponse.WebHanlder
}

// NewHandler creates a v2 response Handler with the given registry,
// default renderer, and legacy handler for fallback.
func NewHandler(registry Registry, defaultRenderer renderers.Renderer, legacyHandler legacyResponse.WebHanlder) *Handler {
	if defaultRenderer == nil {
		defaultRenderer = renderers.JSONRenderer{}
	}
	if registry == nil {
		registry = NewRegistry(nil)
	}
	return &Handler{
		registry:      registry,
		defaultRend:   defaultRenderer,
		legacyHandler: legacyHandler,
	}
}

// Registry returns the error handler registry.
func (h *Handler) Registry() Registry {
	return h.registry
}

// DefaultRenderer returns the default renderer.
func (h *Handler) DefaultRenderer() renderers.Renderer {
	return h.defaultRend
}

// LegacyHandler returns the v1 WebHanlder for fallback/delegation.
func (h *Handler) LegacyHandler() legacyResponse.WebHanlder {
	return h.legacyHandler
}

// commit sends a response through the parser after running before-commit
// hooks. It marks the context committed and emits a mandatory AddLog failure
// entry if the parser write fails.
func (h *Handler) commit(req *v2wf.RequestContext, status int, contentType string, body []byte) error {
	if req == nil {
		return errors.New("response: nil request context")
	}
	if req.Parser == nil {
		return errors.New("response: nil parser")
	}
	// Run before-commit hooks (session cookie persistence, etc.).
	// Hook errors are logged but do not block the commit.
	if hookErr := req.RunBeforeCommitHooks(); hookErr != nil {
		addLogFailure(req, "response-commit-hook", hookErr)
	}
	if err := req.Parser.SendResponse(status, contentType, body); err != nil {
		addLogFailure(req, "response-write", err)
		return err
	}
	req.MarkCommitted(status)
	return nil
}

// OK sends a successful response with the default renderer at HTTP 200.
func (h *Handler) OK(req *v2wf.RequestContext, data any) error {
	return h.OKWithRenderer(req, h.defaultRend, data)
}

// OKWithRenderer sends a successful response with the given renderer at HTTP 200.
func (h *Handler) OKWithRenderer(req *v2wf.RequestContext, renderer renderers.Renderer, data any) error {
	if renderer == nil {
		renderer = h.defaultRend
	}
	payload, err := renderer.Encode(data)
	if err != nil {
		addLogFailure(req, "renderer-encode", err)
		return h.Error(req, err)
	}
	return h.commit(req, http.StatusOK, renderer.ContentType(), payload)
}

// OKWithStatus sends a successful response with the default renderer at the given status.
func (h *Handler) OKWithStatus(req *v2wf.RequestContext, status int, data any) error {
	return h.OKWithStatusAndRenderer(req, status, h.defaultRend, data)
}

// OKWithStatusAndRenderer sends a successful response with the given renderer and status.
func (h *Handler) OKWithStatusAndRenderer(req *v2wf.RequestContext, status int, renderer renderers.Renderer, data any) error {
	if renderer == nil {
		renderer = h.defaultRend
	}
	payload, err := renderer.Encode(data)
	if err != nil {
		addLogFailure(req, "renderer-encode", err)
		return h.Error(req, err)
	}
	return h.commit(req, status, renderer.ContentType(), payload)
}

// Error handles an error through the error handler registry.
func (h *Handler) Error(req *v2wf.RequestContext, err error) error {
	if err == nil {
		return nil
	}
	if req == nil {
		return errors.New("response: nil request context for error")
	}
	// If the response is already committed, we cannot write an error
	// response. Log the failure and return the original error.
	if req.Committed() {
		addLogFailure(req, "error-after-commit", err)
		return err
	}
	return h.registry.Handle(req, err)
}

// NoContent sends a 204 No Content response.
func (h *Handler) NoContent(req *v2wf.RequestContext) error {
	return h.commit(req, http.StatusNoContent, "", nil)
}

// Redirect sends a redirect response.
func (h *Handler) Redirect(req *v2wf.RequestContext, status int, url string) error {
	if req.Legacy.Parser != nil {
		req.Legacy.Parser.SetRespHeader("Location", url)
	}
	return h.commit(req, status, "", nil)
}

// addLogFailure emits a mandatory AddLog failure entry for the given source
// and error. The log key follows the convention "<source>-req-failed".
// This ensures renderer-encode, response-write, and commit-hook failures
// flow into the Splunk transaction pipeline.
func addLogFailure(req *v2wf.RequestContext, source string, err error) {
	if req == nil || req.Legacy.Parser == nil {
		return
	}
	w := webFramework.WebFramework{Parser: req.Legacy.Parser}
	webFramework.AddLog(w, source+"-req-failed", slog.Any("error", err))
}

// SanitizeError converts an arbitrary error into a libError.ErrorData with
// InternalServerError status if it is not already a known error type. This
// is used by fallback handlers to avoid leaking raw error strings.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	var libErr legacyError.ErrorData
	if errors.As(err, &libErr) {
		return err
	}
	var state legacyResponse.ErrorState
	if errors.As(err, &state) {
		return err
	}
	return legacyError.NewWithDescription(
		status.InternalServerError,
		legacyResponse.SystemFault,
		"internal server error",
	)
}
