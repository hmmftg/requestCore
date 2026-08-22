package response

import (
	"encoding/json"
	"errors"
	"net/http"

	legacyResponse "github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/webFramework"
)

// ErrorResponseEntry represents a single error in a structured error response.
type ErrorResponseEntry struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// ErrorResponse is the standard v2 error response body.
type ErrorResponse struct {
	Errors []ErrorResponseEntry `json:"errors"`
}

// DefaultErrorHandlers returns a map of default error handlers for common
// HTTP status codes. These are opt-in presets; applications can register
// them individually or override with custom handlers.
//
// Each handler writes a structured JSON error response using the JSONRenderer
// and logs the error through webFramework.AddLog via the legacy WebFramework.
func DefaultErrorHandlers() map[int]webFramework.ErrorHandler {
	r := renderers.JSONRenderer{}
	return map[int]webFramework.ErrorHandler{
		http.StatusUnauthorized: makeDefaultHandler(http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", r),
		http.StatusForbidden:    makeDefaultHandler(http.StatusForbidden, "FORBIDDEN", "Permission denied", r),
		http.StatusNotFound:     makeDefaultHandler(http.StatusNotFound, "NOT_FOUND", "Resource not found", r),
		http.StatusTooManyRequests: func(ctx webFramework.ErrorContext) error {
			body := ErrorResponse{
				Errors: []ErrorResponseEntry{
					{Code: "RATE_LIMITED", Description: "Too many requests"},
				},
			}
			payload, err := r.Encode(body)
			if err != nil {
				return err
			}
			if ctx.Request != nil && ctx.Request.Legacy.Parser != nil {
				ctx.Request.Legacy.Parser.SetRespHeader("Retry-After", "60")
			}
			return ctx.Request.Parser.SendResponse(ctx.Status, r.ContentType(), payload)
		},
		http.StatusInternalServerError: makeDefaultHandler(http.StatusInternalServerError, "SYSTEM_FAULT", "Internal server error", r),
	}
}

func makeDefaultHandler(status int, code, description string, r renderers.Renderer) webFramework.ErrorHandler {
	return func(ctx webFramework.ErrorContext) error {
		body := ErrorResponse{
			Errors: []ErrorResponseEntry{
				{Code: code, Description: description},
			},
		}
		payload, err := r.Encode(body)
		if err != nil {
			return err
		}
		return ctx.Request.Parser.SendResponse(status, r.ContentType(), payload)
	}
}

// LegacyFallback creates a fallback error handler that delegates to the
// v1 response.WebHanlder.Error method. This preserves the existing
// localization, sanitization, and Splunk transaction pipeline behavior.
//
// The legacyHandler must be non-nil. The legacyWebFramework is extracted
// from the RequestContext.
func LegacyFallback(legacyHandler legacyResponse.WebHanlder) webFramework.ErrorHandler {
	return func(ctx webFramework.ErrorContext) error {
		if ctx.Request == nil {
			return errors.New("response: nil request context in fallback")
		}
		legacyHandler.Error(ctx.Request.Legacy, ctx.Error)
		return nil
	}
}

// EnsureJSONErrorResponse is a helper that encodes an error response as JSON
// and sends it through the v2 parser. It is used by custom error handlers
// that need to send structured error responses.
func EnsureJSONErrorResponse(req *webFramework.RequestContext, status int, code, description string) error {
	r := renderers.JSONRenderer{}
	body := ErrorResponse{
		Errors: []ErrorResponseEntry{
			{Code: code, Description: description},
		},
	}
	payload, err := r.Encode(body)
	if err != nil {
		return err
	}
	return req.Parser.SendResponse(status, r.ContentType(), payload)
}

// EnsureErrorResponse sends a structured error response using the given renderer.
func EnsureErrorResponse(req *webFramework.RequestContext, status int, renderer renderers.Renderer, body any) error {
	payload, err := renderer.Encode(body)
	if err != nil {
		return err
	}
	return req.Parser.SendResponse(status, renderer.ContentType(), payload)
}

// MarshalErrorResponse encodes an ErrorResponse to JSON bytes.
// Useful for testing.
func MarshalErrorResponse(body ErrorResponse) ([]byte, error) {
	return json.Marshal(body)
}
