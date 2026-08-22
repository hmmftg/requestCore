package response

import (
	"encoding/json"
	"errors"
	"net/http"

	legacyResponse "github.com/hmmftg/requestCore/response"

	"github.com/hmmftg/requestCore/v2/renderers"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
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
// and emits a mandatory webFramework.AddLog failure entry so the error
// flows into the Splunk transaction pipeline.
func DefaultErrorHandlers() map[int]v2wf.ErrorHandler {
	r := renderers.JSONRenderer{}
	return map[int]v2wf.ErrorHandler{
		http.StatusUnauthorized: makeDefaultHandler(http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", r),
		http.StatusForbidden:    makeDefaultHandler(http.StatusForbidden, "FORBIDDEN", "Permission denied", r),
		http.StatusNotFound:     makeDefaultHandler(http.StatusNotFound, "NOT_FOUND", "Resource not found", r),
		http.StatusTooManyRequests: func(ctx v2wf.ErrorContext) error {
			body := ErrorResponse{
				Errors: []ErrorResponseEntry{
					{Code: "RATE_LIMITED", Description: "Too many requests"},
				},
			}
			payload, err := r.Encode(body)
			if err != nil {
				addLogFailure(ctx.Request, "default-error-encode", err)
				return err
			}
			if ctx.Request != nil && ctx.Request.Legacy.Parser != nil {
				ctx.Request.Legacy.Parser.SetRespHeader("Retry-After", "60")
			}
			addLogFailure(ctx.Request, "default-error", ctx.Error)
			if err := commitError(ctx.Request, ctx.Status, r.ContentType(), payload); err != nil {
				return err
			}
			return nil
		},
		http.StatusInternalServerError: makeDefaultHandler(http.StatusInternalServerError, "SYSTEM_FAULT", "Internal server error", r),
	}
}

func makeDefaultHandler(status int, code, description string, r renderers.Renderer) v2wf.ErrorHandler {
	return func(ctx v2wf.ErrorContext) error {
		body := ErrorResponse{
			Errors: []ErrorResponseEntry{
				{Code: code, Description: description},
			},
		}
		payload, err := r.Encode(body)
		if err != nil {
			addLogFailure(ctx.Request, "default-error-encode", err)
			return err
		}
		addLogFailure(ctx.Request, "default-error", ctx.Error)
		return commitError(ctx.Request, status, r.ContentType(), payload)
	}
}

// commitError writes an error response through the parser after running
// before-commit hooks and marking the context committed.
func commitError(req *v2wf.RequestContext, status int, contentType string, body []byte) error {
	if req == nil || req.Parser == nil {
		return errors.New("response: nil request context or parser")
	}
	if hookErr := req.RunBeforeCommitHooks(); hookErr != nil {
		addLogFailure(req, "error-commit-hook", hookErr)
	}
	if err := req.Parser.SendResponse(status, contentType, body); err != nil {
		addLogFailure(req, "error-write", err)
		return err
	}
	req.MarkCommitted(status)
	return nil
}

// LegacyFallback creates a fallback error handler that delegates to the
// v1 response.WebHanlder.Error method. This preserves the existing
// localization, sanitization, and Splunk transaction pipeline behavior.
//
// The legacyHandler must be non-nil. The legacyWebFramework is extracted
// from the RequestContext.
func LegacyFallback(legacyHandler legacyResponse.WebHanlder) v2wf.ErrorHandler {
	return func(ctx v2wf.ErrorContext) error {
		if ctx.Request == nil {
			return errors.New("response: nil request context in fallback")
		}
		// Emit a mandatory AddLog failure entry before delegating so
		// the error is captured in the Splunk transaction pipeline.
		addLogFailure(ctx.Request, "legacy-fallback", ctx.Error)
		legacyHandler.Error(ctx.Request.Legacy, ctx.Error)
		// The legacy handler writes its own response; mark committed
		// so the registry does not double-write.
		ctx.Request.MarkCommitted(ctx.Status)
		return nil
	}
}

// EnsureJSONErrorResponse is a helper that encodes an error response as JSON
// and sends it through the v2 parser. It is used by custom error handlers
// that need to send structured error responses.
func EnsureJSONErrorResponse(req *v2wf.RequestContext, status int, code, description string) error {
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
	return commitError(req, status, r.ContentType(), payload)
}

// EnsureErrorResponse sends a structured error response using the given renderer.
func EnsureErrorResponse(req *v2wf.RequestContext, status int, renderer renderers.Renderer, body any) error {
	if renderer == nil {
		renderer = renderers.JSONRenderer{}
	}
	payload, err := renderer.Encode(body)
	if err != nil {
		return err
	}
	return commitError(req, status, renderer.ContentType(), payload)
}

// MarshalErrorResponse encodes an ErrorResponse to JSON bytes.
// Useful for testing.
func MarshalErrorResponse(body ErrorResponse) ([]byte, error) {
	return json.Marshal(body)
}
