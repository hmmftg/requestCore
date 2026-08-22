package response

import (
	"net/http"

	legacyResponse "github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/webFramework"
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
	registry     Registry
	defaultRend  renderers.Renderer
	legacyHandler legacyResponse.WebHanlder
}

// NewHandler creates a v2 response Handler with the given registry,
// default renderer, and legacy handler for fallback.
func NewHandler(registry Registry, defaultRenderer renderers.Renderer, legacyHandler legacyResponse.WebHanlder) *Handler {
	if defaultRenderer == nil {
		defaultRenderer = renderers.JSONRenderer{}
	}
	return &Handler{
		registry:     registry,
		defaultRend:  defaultRenderer,
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

// OK sends a successful response with the default renderer at HTTP 200.
func (h *Handler) OK(req *webFramework.RequestContext, data any) error {
	return h.OKWithRenderer(req, h.defaultRend, data)
}

// OKWithRenderer sends a successful response with the given renderer at HTTP 200.
func (h *Handler) OKWithRenderer(req *webFramework.RequestContext, renderer renderers.Renderer, data any) error {
	payload, err := renderer.Encode(data)
	if err != nil {
		return h.Error(req, err)
	}
	return req.Parser.SendResponse(http.StatusOK, renderer.ContentType(), payload)
}

// OKWithStatus sends a successful response with the default renderer at the given status.
func (h *Handler) OKWithStatus(req *webFramework.RequestContext, status int, data any) error {
	return h.OKWithStatusAndRenderer(req, status, h.defaultRend, data)
}

// OKWithStatusAndRenderer sends a successful response with the given renderer and status.
func (h *Handler) OKWithStatusAndRenderer(req *webFramework.RequestContext, status int, renderer renderers.Renderer, data any) error {
	payload, err := renderer.Encode(data)
	if err != nil {
		return h.Error(req, err)
	}
	return req.Parser.SendResponse(status, renderer.ContentType(), payload)
}

// Error handles an error through the error handler registry.
func (h *Handler) Error(req *webFramework.RequestContext, err error) error {
	return h.registry.Handle(req, err)
}

// NoContent sends a 204 No Content response.
func (h *Handler) NoContent(req *webFramework.RequestContext) error {
	return req.Parser.SendResponse(http.StatusNoContent, "", nil)
}

// Redirect sends a redirect response.
func (h *Handler) Redirect(req *webFramework.RequestContext, status int, url string) error {
	if req.Legacy.Parser != nil {
		req.Legacy.Parser.SetRespHeader("Location", url)
	}
	return req.Parser.SendResponse(status, "", nil)
}
