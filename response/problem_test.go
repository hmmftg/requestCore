package response

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/status"
)

func TestNewProblem(t *testing.T) {
	p := NewProblem(http.StatusBadRequest, "Bad Request")
	if p.Type != "about:blank" {
		t.Errorf("Type = %q, want %q", p.Type, "about:blank")
	}
	if p.Title != "Bad Request" {
		t.Errorf("Title = %q, want %q", p.Title, "Bad Request")
	}
	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
}

func TestNewProblemWithCode(t *testing.T) {
	p := NewProblemWithCode(http.StatusConflict, "Conflict", "DUPLICATE")
	if p.Code != "DUPLICATE" {
		t.Errorf("Code = %q, want %q", p.Code, "DUPLICATE")
	}
}

func TestNewValidationProblem(t *testing.T) {
	violations := []ProblemViolation{
		{Field: "email", Rule: "required", Message: "email is required"},
	}
	p := NewValidationProblem(http.StatusUnprocessableEntity, "Validation Failed", violations)
	if len(p.Violations) != 1 {
		t.Fatalf("Violations len = %d, want 1", len(p.Violations))
	}
	if p.Violations[0].Field != "email" {
		t.Errorf("Violations[0].Field = %q, want %q", p.Violations[0].Field, "email")
	}
}

func TestProblemError(t *testing.T) {
	p := NewProblem(http.StatusNotFound, "Not Found")
	if got := p.Error(); got != "problem: Not Found (404)" {
		t.Errorf("Error() = %q, want %q", got, "problem: Not Found (404)")
	}
}

func TestProblemUnwrap(t *testing.T) {
	cause := libError.ErrorData{}
	p := NewProblem(http.StatusInternalServerError, "Internal Error").WithCause(cause)
	if p.Unwrap() == nil {
		t.Error("Unwrap() = nil, want non-nil")
	}
}

func TestProblemWithDetail(t *testing.T) {
	p := NewProblem(http.StatusBadRequest, "Bad Request").WithDetail("Invalid email format")
	if p.Detail != "Invalid email format" {
		t.Errorf("Detail = %q, want %q", p.Detail, "Invalid email format")
	}
}

func TestProblemWithInstance(t *testing.T) {
	p := NewProblem(http.StatusBadRequest, "Bad Request").WithInstance("/users/123")
	if p.Instance != "/users/123" {
		t.Errorf("Instance = %q, want %q", p.Instance, "/users/123")
	}
}

func TestProblemWithRequestID(t *testing.T) {
	p := NewProblem(http.StatusBadRequest, "Bad Request").WithRequestID("req-123")
	if p.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", p.RequestID, "req-123")
	}
}

func TestProblemMarshalJSON_NoCause(t *testing.T) {
	p := NewProblemWithCode(http.StatusBadRequest, "Bad Request", "BAD_EMAIL").
		WithDetail("Invalid email").
		WithCause(libError.ErrorData{})

	body, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}

	// Verify cause is not serialized
	if _, exists := m["cause"]; exists {
		t.Error("cause field found in JSON output")
	}

	// Verify standard fields
	if m["type"] != "about:blank" {
		t.Errorf("type = %v, want %v", m["type"], "about:blank")
	}
	if m["title"] != "Bad Request" {
		t.Errorf("title = %v, want %v", m["title"], "Bad Request")
	}
	if m["status"].(float64) != http.StatusBadRequest {
		t.Errorf("status = %v, want %d", m["status"], http.StatusBadRequest)
	}
	if m["code"] != "BAD_EMAIL" {
		t.Errorf("code = %v, want %v", m["code"], "BAD_EMAIL")
	}
}

func TestProblemMapperRegistry_NilError(t *testing.T) {
	r := NewProblemMapperRegistry()
	if p := r.Map(nil); p != nil {
		t.Error("Map(nil) should return nil")
	}
}

func TestProblemMapperRegistry_AlreadyProblem(t *testing.T) {
	r := NewProblemMapperRegistry()
	original := NewProblem(http.StatusNotFound, "Not Found")
	p := r.Map(original)
	if p != original {
		t.Error("Map should return the same Problem when already a *Problem")
	}
}

func TestProblemMapperRegistry_DefaultFallback(t *testing.T) {
	r := NewProblemMapperRegistry()
	p := r.Map(errSimple("some error"))
	if p.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusInternalServerError)
	}
	if p.Title != "Internal Server Error" {
		t.Errorf("Title = %q, want %q", p.Title, "Internal Server Error")
	}
	if p.Code != "INTERNAL" {
		t.Errorf("Code = %q, want %q", p.Code, "INTERNAL")
	}
}

func TestProblemMapperRegistry_CustomMapper(t *testing.T) {
	r := NewProblemMapperRegistry()
	err := r.Register(
		func(err error) bool { return true },
		func(err error) *Problem {
			return NewProblem(http.StatusTeapot, "I'm a teapot")
		},
	)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	p := r.Map(errSimple("test"))
	if p.Status != http.StatusTeapot {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusTeapot)
	}
}

func TestProblemMapperRegistry_Freeze(t *testing.T) {
	r := NewProblemMapperRegistry()
	r.Freeze()
	if !r.Frozen() {
		t.Error("Frozen() = false, want true")
	}
	err := r.Register(
		func(err error) bool { return true },
		func(err error) *Problem { return NewProblem(http.StatusTeapot, "teapot") },
	)
	if err != ErrProblemRegistryFrozen {
		t.Errorf("Register() error = %v, want %v", err, ErrProblemRegistryFrozen)
	}
}

func TestDefaultProblemMapperRegistry_LibError(t *testing.T) {
	r := DefaultProblemMapperRegistry()
	err := libError.ErrorData{
		ActionData: libError.Action{
			Status:            status.BadRequest,
			Description:      "BAD_INPUT",
			PublicDescription: "The input was invalid",
		},
	}
	p := r.Map(err)
	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
	if p.Code != "BAD_INPUT" {
		t.Errorf("Code = %q, want %q", p.Code, "BAD_INPUT")
	}
	if p.Detail != "The input was invalid" {
		t.Errorf("Detail = %q, want %q", p.Detail, "The input was invalid")
	}
}

func TestDefaultProblemMapperRegistry_ErrorData(t *testing.T) {
	r := DefaultProblemMapperRegistry()
	err := &ErrorData{
		Status:      http.StatusNotFound,
		Description: "NOT_FOUND",
	}
	p := r.Map(err)
	if p.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusNotFound)
	}
	if p.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want %q", p.Code, "NOT_FOUND")
	}
}

func TestDefaultProblemMapperRegistry_ErrorDataWithViolations(t *testing.T) {
	r := DefaultProblemMapperRegistry()
	err := &ErrorData{
		Status:      http.StatusBadRequest,
		Description: "VALIDATION_ERROR",
		Message: []ErrorResponse{
			{Code: "email", Description: "email is required"},
			{Code: "name", Description: "name is required"},
		},
	}
	p := r.Map(err)
	if p.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusBadRequest)
	}
	if len(p.Violations) != 2 {
		t.Fatalf("Violations len = %d, want 2", len(p.Violations))
	}
	if p.Violations[0].Field != "email" {
		t.Errorf("Violations[0].Field = %q, want %q", p.Violations[0].Field, "email")
	}
}

func TestDefaultProblemMapperRegistry_UnknownError(t *testing.T) {
	r := DefaultProblemMapperRegistry()
	p := r.Map(errSimple("unknown error"))
	if p.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", p.Status, http.StatusInternalServerError)
	}
	// Detail should not contain the raw error
	if p.Detail != "" {
		t.Errorf("Detail = %q, want empty (sanitized)", p.Detail)
	}
}

func TestHTTPStatusTitle(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "Bad Request"},
		{http.StatusUnauthorized, "Unauthorized"},
		{http.StatusForbidden, "Forbidden"},
		{http.StatusNotFound, "Not Found"},
		{http.StatusConflict, "Conflict"},
		{http.StatusInternalServerError, "Internal Server Error"},
		{http.StatusTooManyRequests, "Too Many Requests"},
		{http.StatusPreconditionFailed, "Precondition Failed"},
		{http.StatusPreconditionRequired, "Precondition Required"},
		{999, "Error"},
	}
	for _, tt := range tests {
		if got := httpStatusTitle(tt.status); got != tt.want {
			t.Errorf("httpStatusTitle(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

type errSimple string

func (e errSimple) Error() string { return string(e) }
