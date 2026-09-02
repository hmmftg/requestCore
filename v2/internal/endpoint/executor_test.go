package endpoint

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/request/faketransport"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/telemetry"
	"github.com/hmmftg/requestCore/v2/validation"
)

// newTestExecutor creates an executor with a fresh registry and the
// default problem mapper.
func newTestExecutor() *Executor {
	return NewExecutor(WithRegistry(operation.NewRegistry()))
}

// makeCreateUserEndpoint builds a standard create-user endpoint with
// JSON binding and validation.
func makeCreateUserEndpoint() *Endpoint[CreateUserReq, CreateUserResp] {
	return New[CreateUserReq, CreateUserResp](
		func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
			return CreateUserResp{
				ID:    "user-1",
				Name:  req.Name,
				Email: req.Email,
			}, nil
		},
		WithOperation(operation.Operation{ID: "createUser", Method: "POST", Pattern: "/users"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithValidator(validation.New()),
		WithSuccessStatus(http.StatusCreated),
	)
}

func TestExecute_Success(t *testing.T) {
	exec := newTestExecutor()
	ep := makeCreateUserEndpoint()
	body := `{"name":"Alice","email":"alice@example.com","age":30}`
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	resp, err := Execute(exec, ft.Context(), ep, transport)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.ID != "user-1" {
		t.Errorf("resp.ID = %q", resp.ID)
	}
	if ft.ResponseStatus() != http.StatusCreated {
		t.Errorf("status = %d, want %d", ft.ResponseStatus(), http.StatusCreated)
	}
	ct := ft.ResponseHeaders().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	var respBody CreateUserResp
	if err := json.Unmarshal(ft.ResponseBody(), &respBody); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if respBody.Name != "Alice" {
		t.Errorf("respBody.Name = %q", respBody.Name)
	}
}

func TestExecute_BindError_InvalidJSON(t *testing.T) {
	exec := newTestExecutor()
	ep := makeCreateUserEndpoint()
	body := `{not valid json`
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, binding.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
	if ft.ResponseStatus() != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", ft.ResponseStatus())
	}
	// Verify problem+json content type.
	ct := ft.ResponseHeaders().Get("Content-Type")
	if !strings.Contains(ct, response.ProblemContentType) {
		t.Errorf("Content-Type = %q, want problem+json", ct)
	}
}

func TestExecute_BindError_BodyTooLarge(t *testing.T) {
	exec := newTestExecutor()
	ep := New[CreateUserReq, CreateUserResp](
		func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
			return CreateUserResp{}, nil
		},
		WithOperation(operation.Operation{ID: "createUser", Method: "POST", Pattern: "/users"}),
		WithBindingPlan(binding.Plan{Mode: binding.ModeJSON, MaxBodyBytes: 10}),
	)
	body := strings.Repeat("a", 100)
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if !errors.Is(err, binding.ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}
	if ft.ResponseStatus() != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", ft.ResponseStatus())
	}
}

func TestExecute_ValidationError(t *testing.T) {
	exec := newTestExecutor()
	ep := makeCreateUserEndpoint()
	// Missing required fields name and email.
	body := `{"age":30}`
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if ft.ResponseStatus() != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", ft.ResponseStatus())
	}
	// Verify response body contains violations.
	var problem response.Problem
	if err := json.Unmarshal(ft.ResponseBody(), &problem); err != nil {
		t.Fatalf("unmarshal problem: %v", err)
	}
	if len(problem.Violations) == 0 {
		t.Error("expected violations in problem, got none")
	}
}

func TestExecute_HandlerError_MappedProblem(t *testing.T) {
	exec := newTestExecutor()
	// Register a custom mapper for our sentinel error.
	exec.ProblemMapper = response.NewMapperRegistry()
	notFoundErr := errors.New("user not found")
	exec.ProblemMapper.Register(
		func(err error) bool { return errors.Is(err, notFoundErr) },
		func(err error) *response.Problem {
			return response.NewProblemWithCode(http.StatusNotFound, "User Not Found", "USER_NOT_FOUND")
		},
	)

	ep := New[problemReq, problemResp](
		func(ctx *request.Context, req problemReq) (problemResp, error) {
			if req.Mode == "notfound" {
				return problemResp{}, notFoundErr
			}
			return problemResp{Result: "ok"}, nil
		},
		WithOperation(operation.Operation{ID: "testProblem", Method: "POST", Pattern: "/test"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithValidator(validation.New()),
	)

	body := `{"mode":"notfound"}`
	ft := faketransport.New("POST", "/test", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if ft.ResponseStatus() != http.StatusNotFound {
		t.Errorf("status = %d, want 404", ft.ResponseStatus())
	}
}

func TestExecute_HandlerPanic_Recovered(t *testing.T) {
	exec := newTestExecutor()
	ep := New[problemReq, problemResp](
		func(ctx *request.Context, req problemReq) (problemResp, error) {
			panic("boom")
		},
		WithOperation(operation.Operation{ID: "testPanic", Method: "POST", Pattern: "/panic"}),
		WithBindingPlan(binding.DefaultJSONPlan),
		WithValidator(validation.New()),
	)

	body := `{"mode":"ok"}`
	ft := faketransport.New("POST", "/panic", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}
	// Panic should map to 500 via default sanitizer.
	if ft.ResponseStatus() != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", ft.ResponseStatus())
	}
}

func TestExecute_NoBinding(t *testing.T) {
	exec := newTestExecutor()
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "pong"}, nil
		},
		WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		// No binding plan — defaults to ModeNone.
	)

	ft := faketransport.New("GET", "/ping")
	transport := &FakeTransportAdapter{FT: ft}

	resp, err := Execute(exec, ft.Context(), ep, transport)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if resp.Message != "pong" {
		t.Errorf("Message = %q", resp.Message)
	}
	if ft.ResponseStatus() != http.StatusOK {
		t.Errorf("status = %d, want 200", ft.ResponseStatus())
	}
}

func TestExecute_EmptyBody(t *testing.T) {
	exec := newTestExecutor()
	ep := makeCreateUserEndpoint()
	// Empty body — binding succeeds with zero values, validation fails.
	ft := faketransport.New("POST", "/users", faketransport.WithBody(""))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected validation error for empty body, got nil")
	}
	if ft.ResponseStatus() != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", ft.ResponseStatus())
	}
}

func TestExecute_TelemetryEmitted(t *testing.T) {
	// Use a capturing sink to verify events.
	sink := &capturingSink{}
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithTelemetrySink(sink),
	)
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "pong"}, nil
		},
		WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)

	ft := faketransport.New("GET", "/ping")
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("expected 2 events (start+success), got %d", len(sink.events))
	}
	if sink.events[0].Type != telemetry.EventStart {
		t.Errorf("event 0 type = %v, want start", sink.events[0].Type)
	}
	if sink.events[1].Type != telemetry.EventSuccess {
		t.Errorf("event 1 type = %v, want success", sink.events[1].Type)
	}
	if sink.events[1].Status != http.StatusOK {
		t.Errorf("event 1 status = %d, want 200", sink.events[1].Status)
	}
}

func TestExecute_TelemetryOnFailure(t *testing.T) {
	sink := &capturingSink{}
	exec := NewExecutor(
		WithRegistry(operation.NewRegistry()),
		WithTelemetrySink(sink),
	)
	ep := makeCreateUserEndpoint()
	body := `{invalid`
	ft := faketransport.New("POST", "/users", faketransport.WithBody(body))
	transport := &FakeTransportAdapter{FT: ft}

	_, err := Execute(exec, ft.Context(), ep, transport)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(sink.events) != 2 {
		t.Fatalf("expected 2 events (start+failure), got %d", len(sink.events))
	}
	if sink.events[1].Type != telemetry.EventFailure {
		t.Errorf("event 1 type = %v, want failure", sink.events[1].Type)
	}
	if sink.events[1].Err == nil {
		t.Error("expected error in failure event")
	}
}

func TestRegister_DuplicateID(t *testing.T) {
	exec := newTestExecutor()
	ep1 := makeCreateUserEndpoint()
	ep2 := makeCreateUserEndpoint()
	if err := Register(exec, ep1); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := Register(exec, ep2); err == nil {
		t.Fatal("expected duplicate ID error, got nil")
	}
}

func TestRegister_Success(t *testing.T) {
	exec := newTestExecutor()
	ep := makeCreateUserEndpoint()
	if err := Register(exec, ep); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	op, ok := exec.Registry.Get("createUser")
	if !ok {
		t.Fatal("operation not found in registry")
	}
	if op.Method != "POST" {
		t.Errorf("Method = %q", op.Method)
	}
}

func TestExecute_SuccessStatus(t *testing.T) {
	exec := newTestExecutor()
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "created"}, nil
		},
		WithOperation(operation.Operation{ID: "create", Method: "POST", Pattern: "/create"}),
		WithSuccessStatus(http.StatusAccepted),
	)
	ft := faketransport.New("POST", "/create")
	transport := &FakeTransportAdapter{FT: ft}
	_, err := Execute(exec, ft.Context(), ep, transport)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if ft.ResponseStatus() != http.StatusAccepted {
		t.Errorf("status = %d, want 202", ft.ResponseStatus())
	}
}

func TestExecute_SuccessContentType(t *testing.T) {
	exec := newTestExecutor()
	ep := New[PingReq, PingResp](
		func(ctx *request.Context, req PingReq) (PingResp, error) {
			return PingResp{Message: "pong"}, nil
		},
		WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		WithSuccessContentType("application/vnd.custom+json"),
	)
	ft := faketransport.New("GET", "/ping")
	transport := &FakeTransportAdapter{FT: ft}
	_, err := Execute(exec, ft.Context(), ep, transport)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	ct := ft.ResponseHeaders().Get("Content-Type")
	if ct != "application/vnd.custom+json" {
		t.Errorf("Content-Type = %q, want application/vnd.custom+json", ct)
	}
}

// capturingSink is a test telemetry sink that records events.
type capturingSink struct {
	events []telemetry.Event
}

func (s *capturingSink) Record(e telemetry.Event) {
	s.events = append(s.events, e)
}
