package adapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/v2/endpoint"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
)

type pingReq struct{}
type pingResp struct {
	Message string `json:"message"`
}

func TestAdapter_Wrap_Success(t *testing.T) {
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{Message: "pong"}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)

	router := v2libChi.NewRouter()
	router.SetErrorHandler(MappedErrorHandler(response.DefaultMapperRegistry()))

	if err := Register(router, exec, "GET", "/ping", ep); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result pingResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if result.Message != "pong" {
		t.Fatalf("expected 'pong', got %q", result.Message)
	}
}

func TestAdapter_Wrap_HandlerError(t *testing.T) {
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{}, errors.New("something went wrong")
		},
		endpoint.WithOperation(operation.Operation{ID: "fail", Method: "GET", Pattern: "/fail"}),
	)

	router := v2libChi.NewRouter()
	router.SetErrorHandler(MappedErrorHandler(response.DefaultMapperRegistry()))

	if err := Register(router, exec, "GET", "/fail", ep); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	srv := httptest.NewServer(router.Native().(http.Handler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/fail")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestAdapter_Register_RollbackOnRouterFailure(t *testing.T) {
	exec := endpoint.NewExecutor(
		endpoint.WithRegistry(operation.NewRegistry()),
		endpoint.WithNopTelemetry(),
	)
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{Message: "pong"}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "rollback", Method: "GET", Pattern: "/rollback"}),
	)

	// Use a failing router that always returns an error on Handle.
	router := &failingRouter{}

	err := Register(router, exec, "GET", "/rollback", ep)
	if err == nil {
		t.Fatal("expected error from router failure")
	}

	// Verify the operation was rolled back.
	_, ok := exec.Registry.Get("rollback")
	if ok {
		t.Fatal("expected operation to be unregistered after rollback")
	}
}

type failingRouter struct{}

func (r *failingRouter) Group(prefix string) routing.RouteGroup           { return r }
func (r *failingRouter) With(mw ...routing.Middleware) routing.RouteGroup { return r }
func (r *failingRouter) Handle(method, pattern string, handler routing.Handler) error {
	return errors.New("router registration failed")
}
func (r *failingRouter) Get(pattern string, handler routing.Handler) error {
	return r.Handle("GET", pattern, handler)
}
func (r *failingRouter) Post(pattern string, handler routing.Handler) error {
	return r.Handle("POST", pattern, handler)
}
func (r *failingRouter) Put(pattern string, handler routing.Handler) error {
	return r.Handle("PUT", pattern, handler)
}
func (r *failingRouter) Patch(pattern string, handler routing.Handler) error {
	return r.Handle("PATCH", pattern, handler)
}
func (r *failingRouter) Delete(pattern string, handler routing.Handler) error {
	return r.Handle("DELETE", pattern, handler)
}
func (r *failingRouter) Head(pattern string, handler routing.Handler) error {
	return r.Handle("HEAD", pattern, handler)
}

func TestAdapter_MappedErrorHandler(t *testing.T) {
	mapper := response.NewMapperRegistry()
	_ = mapper.Register(
		func(err error) bool {
			var ce *customError
			return errors.As(err, &ce)
		},
		func(err error) *response.Problem {
			return response.NewProblemWithCode(http.StatusBadRequest, "Bad Request", "CUSTOM_ERROR")
		},
	)

	handler := MappedErrorHandler(mapper)

	// Create a fake transport to capture the response.
	rec := httptest.NewRecorder()
	transport := &testTransport{w: rec}

	ctx := request.NewContext(nil)
	handler(ctx, transport, &customError{msg: "test error"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}
}

type customError struct{ msg string }

func (e *customError) Error() string { return e.msg }

type testTransport struct {
	w         http.ResponseWriter
	committed bool
}

func (t *testTransport) WriteResponse(status int, ct string, headers http.Header, body []byte) error {
	t.committed = true
	for k, vs := range headers {
		for _, v := range vs {
			t.w.Header().Add(k, v)
		}
	}
	if ct != "" {
		t.w.Header().Set("Content-Type", ct)
	}
	t.w.WriteHeader(status)
	_, err := t.w.Write(body)
	return err
}

func (t *testTransport) Committed() bool { return t.committed }
