package response

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/webFramework"
)

// ProblemContentType is the media type for RFC 9457 problem details.
const ProblemContentType = "application/problem+json"

// ProblemViolation represents a single validation violation in a
// Problem response.
type ProblemViolation struct {
	// Field is the JSON/query/path/header field name that violated a
	// constraint. Uses the wire name, not the Go struct field name.
	Field string `json:"field"`

	// Rule is the validation rule that was violated (e.g. "required",
	// "min", "max_length").
	Rule string `json:"rule"`

	// Message is a human-readable description of the violation.
	Message string `json:"message"`
}

// Problem implements RFC 9457 Problem Details for HTTP APIs. It
// satisfies the error and Unwrap interfaces.
//
// This is an additive, opt-in facility. The default v1 response path
// continues to use the legacy WsResponse envelope. Problem is intended
// for handlers that explicitly opt into RFC 9457 error responses.
//
// Causes are never serialized by default. Unknown errors always become
// sanitized 500 problems via the mapper registry.
type Problem struct {
	// Type is a URI reference identifying the problem type. Defaults to
	// "about:blank" per RFC 9457.
	Type string `json:"type"`

	// Title is a short, human-readable summary of the problem type.
	Title string `json:"title"`

	// Status is the HTTP status code.
	Status int `json:"status"`

	// Detail is a human-readable, safe explanation specific to this
	// occurrence. Must not leak sensitive data.
	Detail string `json:"detail,omitempty"`

	// Instance is a URI reference identifying the specific occurrence.
	Instance string `json:"instance,omitempty"`

	// Code is a stable extension identifying the error code.
	Code string `json:"code,omitempty"`

	// Violations is an extension listing validation violations.
	Violations []ProblemViolation `json:"violations,omitempty"`

	// RequestID is an extension carrying the request identifier.
	RequestID string `json:"request_id,omitempty"`

	// TraceID is an extension carrying the trace identifier.
	TraceID string `json:"trace_id,omitempty"`

	// cause is the underlying error, never serialized by default.
	cause error
}

// NewProblem creates a Problem with the given status and title. The
// Type defaults to "about:blank".
func NewProblem(status int, title string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
	}
}

// NewProblemWithCode creates a Problem with a stable error code extension.
func NewProblemWithCode(status int, title, code string) *Problem {
	return &Problem{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Code:   code,
	}
}

// NewValidationProblem creates a Problem with validation violations.
// The status should typically be 422 (Unprocessable Entity) or 400.
func NewValidationProblem(status int, title string, violations []ProblemViolation) *Problem {
	return &Problem{
		Type:       "about:blank",
		Title:      title,
		Status:     status,
		Violations: violations,
	}
}

// Error implements the error interface. It returns a string in the
// format "problem: <title> (<status>)" without exposing the cause.
func (p *Problem) Error() string {
	return fmt.Sprintf("problem: %s (%d)", p.Title, p.Status)
}

// Unwrap returns the underlying cause error, or nil if none was set.
// This supports errors.Is and errors.As chaining.
func (p *Problem) Unwrap() error {
	return p.cause
}

// WithDetail sets the detail and returns the problem for chaining.
func (p *Problem) WithDetail(detail string) *Problem {
	p.Detail = detail
	return p
}

// WithInstance sets the instance URI and returns the problem for chaining.
func (p *Problem) WithInstance(instance string) *Problem {
	p.Instance = instance
	return p
}

// WithRequestID sets the request ID extension and returns the problem.
func (p *Problem) WithRequestID(id string) *Problem {
	p.RequestID = id
	return p
}

// WithTraceID sets the trace ID extension and returns the problem.
func (p *Problem) WithTraceID(id string) *Problem {
	p.TraceID = id
	return p
}

// WithCode sets the stable error code extension and returns the problem.
func (p *Problem) WithCode(code string) *Problem {
	p.Code = code
	return p
}

// WithCause sets the underlying cause error. The cause is never
// serialized by MarshalJSON. It is accessible via Unwrap for
// errors.Is/errors.As chaining.
func (p *Problem) WithCause(err error) *Problem {
	p.cause = err
	return p
}

// MarshalJSON serializes the Problem as RFC 9457 JSON. The cause is
// never included in the output.
func (p *Problem) MarshalJSON() ([]byte, error) {
	type alias Problem
	return json.Marshal((*alias)(p))
}

// WriteToWebFramework writes the Problem as JSON with the correct
// content type through the v1 webFramework. It sets the Content-Type
// header, records the HTTP status in local storage (consistent with
// the legacy respond path), and sends the JSON body via the parser.
//
// This is the v1 integration point for opt-in RFC 9457 responses.
func (p *Problem) WriteToWebFramework(w webFramework.WebFramework) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("problem: marshal: %w", err)
	}
	w.Parser.SetLocal(LastHTTPStatusLocal, p.Status)
	w.Parser.SetRespHeader("Content-Type", ProblemContentType)
	return w.Parser.SendJSONRespBody(p.Status, json.RawMessage(body))
}

// WriteTo writes the Problem as JSON with the correct content type to
// the given http.ResponseWriter. This is provided for direct net/http
// integration outside the webFramework lifecycle.
func (p *Problem) WriteTo(rw http.ResponseWriter) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("problem: marshal: %w", err)
	}
	rw.Header().Set("Content-Type", ProblemContentType)
	rw.WriteHeader(p.Status)
	_, err = rw.Write(body)
	return err
}

// logValue returns a slog.Value that redacts the cause and any
// sensitive detail. Only status, title, code, and type are logged.
func (p *Problem) logValue() slog.Value {
	return slog.GroupValue(
		slog.String("type", p.Type),
		slog.String("title", p.Title),
		slog.Int("status", p.Status),
		slog.String("code", p.Code),
	)
}
