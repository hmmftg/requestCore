package response

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewProblem_Defaults(t *testing.T) {
	p := NewProblem(404, "Not Found")
	if p.Type != "about:blank" {
		t.Fatalf("expected type about:blank, got %q", p.Type)
	}
	if p.Title != "Not Found" {
		t.Fatalf("expected title 'Not Found', got %q", p.Title)
	}
	if p.Status != 404 {
		t.Fatalf("expected status 404, got %d", p.Status)
	}
	if p.Code != "" {
		t.Fatalf("expected empty code, got %q", p.Code)
	}
	if len(p.Violations) != 0 {
		t.Fatalf("expected no violations, got %v", p.Violations)
	}
}

func TestNewProblemWithCode(t *testing.T) {
	p := NewProblemWithCode(403, "Forbidden", "PERMISSION_DENIED")
	if p.Code != "PERMISSION_DENIED" {
		t.Fatalf("expected code PERMISSION_DENIED, got %q", p.Code)
	}
	if p.Status != 403 {
		t.Fatalf("expected status 403, got %d", p.Status)
	}
}

func TestNewValidationProblem(t *testing.T) {
	violations := []Violation{
		{Field: "email", Rule: "required", Message: "email is required"},
		{Field: "age", Rule: "min", Message: "age must be at least 18"},
	}
	p := NewValidationProblem(422, "Validation Failed", violations)
	if p.Status != 422 {
		t.Fatalf("expected status 422, got %d", p.Status)
	}
	if len(p.Violations) != 2 {
		t.Fatalf("expected 2 violations, got %d", len(p.Violations))
	}
	if p.Violations[0].Field != "email" {
		t.Fatalf("expected first violation field 'email', got %q", p.Violations[0].Field)
	}
}

func TestProblem_Error(t *testing.T) {
	p := NewProblem(500, "Internal Server Error")
	if p.Error() != "problem: Internal Server Error (500)" {
		t.Fatalf("unexpected error string: %q", p.Error())
	}
}

func TestProblem_HTTPStatus(t *testing.T) {
	p := NewProblem(429, "Too Many Requests")
	if p.HTTPStatus() != 429 {
		t.Fatalf("expected 429, got %d", p.HTTPStatus())
	}
}

func TestProblem_Unwrap(t *testing.T) {
	cause := errors.New("db connection lost")
	p := NewProblem(500, "Internal Server Error").WithCause(cause)
	if !errors.Is(p, cause) {
		t.Fatal("expected errors.Is to find cause")
	}
}

func TestProblem_Unwrap_Nil(t *testing.T) {
	p := NewProblem(404, "Not Found")
	if p.Unwrap() != nil {
		t.Fatal("expected nil unwrap for problem without cause")
	}
}

func TestProblem_BuilderMethods(t *testing.T) {
	p := NewProblem(400, "Bad Request").
		WithDetail("missing required field").
		WithInstance("/users/42").
		WithRequestID("req-123").
		WithTraceID("trace-456").
		WithCode("MISSING_FIELD")

	if p.Detail != "missing required field" {
		t.Fatalf("expected detail, got %q", p.Detail)
	}
	if p.Instance != "/users/42" {
		t.Fatalf("expected instance, got %q", p.Instance)
	}
	if p.RequestID != "req-123" {
		t.Fatalf("expected request_id, got %q", p.RequestID)
	}
	if p.TraceID != "trace-456" {
		t.Fatalf("expected trace_id, got %q", p.TraceID)
	}
	if p.Code != "MISSING_FIELD" {
		t.Fatalf("expected code, got %q", p.Code)
	}
}

func TestProblem_MarshalJSON(t *testing.T) {
	p := NewProblem(404, "Not Found").
		WithDetail("user not found").
		WithRequestID("req-123").
		WithTraceID("trace-456").
		WithCode("USER_NOT_FOUND").
		WithCause(errors.New("secret internal error"))

	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	str := string(body)

	// Verify required fields.
	if !strings.Contains(str, `"type":"about:blank"`) {
		t.Fatalf("expected type in JSON: %s", str)
	}
	if !strings.Contains(str, `"title":"Not Found"`) {
		t.Fatalf("expected title in JSON: %s", str)
	}
	if !strings.Contains(str, `"status":404`) {
		t.Fatalf("expected status in JSON: %s", str)
	}
	if !strings.Contains(str, `"detail":"user not found"`) {
		t.Fatalf("expected detail in JSON: %s", str)
	}
	if !strings.Contains(str, `"code":"USER_NOT_FOUND"`) {
		t.Fatalf("expected code in JSON: %s", str)
	}
	if !strings.Contains(str, `"request_id":"req-123"`) {
		t.Fatalf("expected request_id in JSON: %s", str)
	}
	if !strings.Contains(str, `"trace_id":"trace-456"`) {
		t.Fatalf("expected trace_id in JSON: %s", str)
	}

	// Verify cause is NOT serialized.
	if strings.Contains(str, "secret internal error") {
		t.Fatalf("cause should not be serialized: %s", str)
	}
}

func TestProblem_MarshalJSON_Violations(t *testing.T) {
	p := NewValidationProblem(422, "Validation Failed", []Violation{
		{Field: "email", Rule: "required", Message: "email is required"},
	})

	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	str := string(body)
	if !strings.Contains(str, `"violations"`) {
		t.Fatalf("expected violations in JSON: %s", str)
	}
	if !strings.Contains(str, `"field":"email"`) {
		t.Fatalf("expected field in JSON: %s", str)
	}
	if !strings.Contains(str, `"rule":"required"`) {
		t.Fatalf("expected rule in JSON: %s", str)
	}
}

func TestProblem_MarshalJSON_OmitsEmptyFields(t *testing.T) {
	p := NewProblem(500, "Internal Server Error")
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	str := string(body)
	// detail, instance, code, violations, request_id, trace_id should
	// be omitted when empty.
	if strings.Contains(str, "detail") {
		t.Fatalf("expected detail omitted: %s", str)
	}
	if strings.Contains(str, "instance") {
		t.Fatalf("expected instance omitted: %s", str)
	}
	if strings.Contains(str, "code") {
		t.Fatalf("expected code omitted: %s", str)
	}
	if strings.Contains(str, "violations") {
		t.Fatalf("expected violations omitted: %s", str)
	}
	if strings.Contains(str, "request_id") {
		t.Fatalf("expected request_id omitted: %s", str)
	}
	if strings.Contains(str, "trace_id") {
		t.Fatalf("expected trace_id omitted: %s", str)
	}
}

func TestProblem_WriteTo(t *testing.T) {
	p := NewProblem(422, "Validation Failed").
		WithDetail("email is required").
		WithCode("VALIDATION_ERROR")

	w := httptest.NewRecorder()
	if err := p.WriteTo(w); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	if w.Code != 422 {
		t.Fatalf("expected status 422, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != ProblemContentType {
		t.Fatalf("expected %s, got %s", ProblemContentType, ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"title":"Validation Failed"`) {
		t.Fatalf("expected title in body: %s", body)
	}
	if !strings.Contains(body, `"code":"VALIDATION_ERROR"`) {
		t.Fatalf("expected code in body: %s", body)
	}
}

func TestProblem_HTTPStatusInterface(t *testing.T) {
	p := NewProblem(418, "I'm a teapot")
	status := p.HTTPStatus()
	if status != 418 {
		t.Fatalf("expected 418 from HTTPStatus, got %d", status)
	}
}
