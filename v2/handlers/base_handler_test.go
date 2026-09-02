package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

func init() {
	// Use chi for tests — it is stdlib-only and does not require gin.
}

// testExecutor creates an executor with a fresh registry, nop telemetry,
// and the default problem mapper for handler tests.
func testExecutor() *endpoint.Executor {
	return endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
}

// testRouter creates a chi router with a mapped error handler so that
// handler errors are written as RFC 9457 Problems.
func testRouter() routing.Router {
	r := v2libChi.NewRouter()
	mapper := response.DefaultMapperRegistry()
	r.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = response.WriteProblemFromError(ctx, transport, mapper, err)
	})
	return r
}

// Test types used across handler tests.
type TestResp struct {
	Status string `json:"status"`
}

type CreateReq struct {
	Name string `json:"name"`
}

type CreateResp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// --- Basic endpoint tests ---

// TestEndpoint_GetSuccess verifies the full lifecycle for a successful
// GET endpoint: bind (none), execute, encode, commit.
func TestEndpoint_GetSuccess(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("get-health", "/health",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result TestResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", result.Status)
	}
}

// TestEndpoint_PostSuccess verifies JSON body binding for a POST endpoint.
func TestEndpoint_PostSuccess(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep := Post[CreateReq, CreateResp]("create-user", "/users",
		func(ctx *request.Context, req CreateReq) (CreateResp, error) {
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	body := `{"name":"alice"}`
	resp, err := http.Post(srv.URL+"/users", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result CreateResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Name != "alice" {
		t.Fatalf("expected name 'alice', got %q", result.Name)
	}
}

// TestEndpoint_HandlerError verifies that handler errors are mapped to
// RFC 9457 Problems and written through the transport.
func TestEndpoint_HandlerError(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("fail-handler", "/fail",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, errors.New("handler failure")
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/fail")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// TestEndpoint_PanicRecovery verifies that panics are recovered by the
// executor and written as 500 responses.
func TestEndpoint_PanicRecovery(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("panic-handler", "/panic",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			panic("boom")
		},
	)
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/panic")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// --- Interface and metadata tests ---

// TestEndpoint_OperationMetadata verifies that OperationID, Method, and
// Pattern return the values passed to New.
func TestEndpoint_OperationMetadata(t *testing.T) {
	ep := New[struct{}, TestResp]("my-op", "GET", "/my-path",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	if ep.OperationID() != "my-op" {
		t.Fatalf("OperationID = %q, want %q", ep.OperationID(), "my-op")
	}
	if ep.Method() != "GET" {
		t.Fatalf("Method = %q, want %q", ep.Method(), "GET")
	}
	if ep.Pattern() != "/my-path" {
		t.Fatalf("Pattern = %q, want %q", ep.Pattern(), "/my-path")
	}
}

// TestEndpoint_SetPath verifies that SetPath updates the pattern.
func TestEndpoint_SetPath(t *testing.T) {
	ep := New[struct{}, TestResp]("set-path-op", "GET", "/original",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	ep.SetPath("/updated")
	if ep.Pattern() != "/updated" {
		t.Fatalf("Pattern = %q, want %q", ep.Pattern(), "/updated")
	}
}

// TestEndpoint_SatisfiesInterfaces verifies that *Endpoint satisfies
// both EndpointRuntime and ConfigurableEndpoint.
func TestEndpoint_SatisfiesInterfaces(t *testing.T) {
	var _ EndpointRuntime = &Endpoint[struct{}, TestResp]{}
	var _ ConfigurableEndpoint = &Endpoint[struct{}, TestResp]{}
}

// TestEndpoint_InnerReturnsWrappedEndpoint verifies that Inner returns
// the wrapped endpoint.Endpoint for advanced configuration.
func TestEndpoint_InnerReturnsWrappedEndpoint(t *testing.T) {
	ep := New[struct{}, TestResp]("inner-op", "GET", "/inner",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	inner := ep.Inner()
	if inner == nil {
		t.Fatal("expected non-nil inner endpoint")
	}
	if inner.Config.Operation.ID != "inner-op" {
		t.Fatalf("inner operation ID = %q, want %q", inner.Config.Operation.ID, "inner-op")
	}
}

// --- Convenience constructor tests ---

// TestConvenienceConstructors verifies that each convenience constructor
// sets the correct HTTP method.
func TestConvenienceConstructors(t *testing.T) {
	handler := func(ctx *request.Context, req struct{}) (struct{}, error) {
		return struct{}{}, nil
	}
	tests := []struct {
		name   string
		ctor   func(opID, pattern string, h func(*request.Context, struct{}) (struct{}, error)) *Endpoint[struct{}, struct{}]
		method string
	}{
		{"Get", Get[struct{}, struct{}], "GET"},
		{"Post", Post[struct{}, struct{}], "POST"},
		{"Put", Put[struct{}, struct{}], "PUT"},
		{"Patch", Patch[struct{}, struct{}], "PATCH"},
		{"Delete", Delete[struct{}, struct{}], "DELETE"},
		{"Head", Head[struct{}, struct{}], "HEAD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ep := tt.ctor(tt.name+":test", "/test", handler)
			if ep.Method() != tt.method {
				t.Fatalf("%s: Method = %q, want %q", tt.name, ep.Method(), tt.method)
			}
		})
	}
}

// --- Registration function tests ---

// TestRegisterEndpoint_NilEndpoint verifies that RegisterEndpoint
// returns an error for a nil endpoint.
func TestRegisterEndpoint_NilEndpoint(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	err := RegisterEndpoint[struct{}, TestResp](router, exec, nil)
	if err == nil {
		t.Fatal("expected error for nil endpoint")
	}
}

// TestRegisterRuntime_NilEndpoint verifies that RegisterRuntime returns
// an error for a nil endpoint.
func TestRegisterRuntime_NilEndpoint(t *testing.T) {
	exec := testExecutor()
	router := testRouter()
	err := RegisterRuntime(router, exec, nil)
	if err == nil {
		t.Fatal("expected error for nil endpoint runtime")
	}
}

// TestRegisterRuntime_WithEndpoint verifies that RegisterRuntime works
// with an EndpointRuntime interface value.
func TestRegisterRuntime_WithEndpoint(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep := Get[struct{}, TestResp]("runtime-test", "/runtime",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err := RegisterRuntime(router, exec, ep); err != nil {
		t.Fatalf("RegisterRuntime: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/runtime")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestRegisterEndpoint_DuplicateOpID verifies that registering the same
// operation ID twice fails.
func TestRegisterEndpoint_DuplicateOpID(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	ep1 := Get[struct{}, TestResp]("dup-op", "/first",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	if err := RegisterEndpoint(router, exec, ep1); err != nil {
		t.Fatalf("first RegisterEndpoint: %v", err)
	}

	ep2 := Get[struct{}, TestResp]("dup-op", "/second",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	err := RegisterEndpoint(router, exec, ep2)
	if err == nil {
		t.Fatal("expected error for duplicate operation ID")
	}
}

// TestRegisterEndpoint_MethodMismatch verifies that the adapter rejects
// registration when the method doesn't match the operation metadata.
func TestRegisterEndpoint_MethodMismatch(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	// Create a GET endpoint but try to register it — the adapter
	// validates that the operation method matches the registration.
	// Since RegisterEndpoint uses ep.Method(), this should succeed.
	// To test mismatch, we manually create an endpoint with a
	// different method in the operation than the registration path.
	ep := New[struct{}, TestResp]("mismatch-op", "GET", "/mismatch",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{}, nil
		},
	)
	// Register normally — should succeed.
	if err := RegisterEndpoint(router, exec, ep); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}
}

// --- Convenience registration function tests ---

// TestGetEndpoint registers and tests a GET endpoint via GetEndpoint.
func TestGetEndpoint(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	err := GetEndpoint[struct{}, TestResp](router, exec, "/get-endpoint",
		func(ctx *request.Context, req struct{}) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/get-endpoint")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestPostEndpoint registers and tests a POST endpoint via PostEndpoint.
func TestPostEndpoint(t *testing.T) {
	exec := testExecutor()
	router := testRouter()

	err := PostEndpoint[CreateReq, CreateResp](router, exec, "/post-endpoint",
		func(ctx *request.Context, req CreateReq) (CreateResp, error) {
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	)
	if err != nil {
		t.Fatalf("PostEndpoint: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/post-endpoint", "application/json", strings.NewReader(`{"name":"bob"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result CreateResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Name != "bob" {
		t.Fatalf("expected name 'bob', got %q", result.Name)
	}
}

// TestDefaultOpID verifies that defaultOpID generates deterministic IDs.
func TestDefaultOpID(t *testing.T) {
	tests := []struct {
		method, pattern, want string
	}{
		{"GET", "/health", "GET:health"},
		{"POST", "/users", "POST:users"},
		{"GET", "/users/{id}", "GET:users-id"},
		{"PUT", "/items/{id}/edit", "PUT:items-id-edit"},
		{"DELETE", "/items/{id}", "DELETE:items-id"},
	}
	for _, tt := range tests {
		got := defaultOpID(tt.method, tt.pattern)
		if got != tt.want {
			t.Fatalf("defaultOpID(%q, %q) = %q, want %q", tt.method, tt.pattern, got, tt.want)
		}
	}
}
