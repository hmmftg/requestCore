package nextadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/endpoint"
	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/telemetry"
	"github.com/hmmftg/requestCore/v2/validation"
)

// newTestExecutor creates an executor with a fresh registry and the
// default problem mapper.
func newTestExecutor() *endpoint.Executor {
	return endpoint.NewExecutor(endpoint.WithRegistry(operation.NewRegistry()), endpoint.WithNopTelemetry())
}

// --- fixture types ---

type createUserReq struct {
	Name  string `json:"name" validate:"required,min=2,max=50"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
}

type createUserResp struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type pingReq struct{}

type pingResp struct {
	Message string `json:"message"`
}

type pathReq struct {
	ID string `path:"id"`
}

type pathResp struct {
	ID string `json:"id"`
}

type queryReq struct {
	Page int `query:"page" validate:"gte=1"`
}

type queryResp struct {
	Page int `json:"page"`
}

type problemReq struct {
	Mode string `json:"mode" validate:"required,oneof=ok notfound"`
}

type problemResp struct {
	Result string `json:"result"`
}

// --- helpers ---

// makeCreateUserEndpoint builds a standard create-user endpoint with
// JSON binding and validation.
func makeCreateUserEndpoint() *endpoint.Endpoint[createUserReq, createUserResp] {
	return endpoint.New[createUserReq, createUserResp](
		func(ctx *request.Context, req createUserReq) (createUserResp, error) {
			return createUserResp{ID: "user-1", Name: req.Name, Email: req.Email}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "createUser", Method: "POST", Pattern: "/users"}),
		endpoint.WithBindingPlan(binding.DefaultJSONPlan),
		endpoint.WithValidator(validation.New()),
		endpoint.WithSuccessStatus(http.StatusCreated),
	)
}

// runChiRequest runs a request through a chi router and returns the
// recorder. The router is created fresh for each call.
func runChiRequest(t *testing.T, register func(routing.RouteGroup, *endpoint.Executor), method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	register(router, exec)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	serveHTTP(t, router, w, req)
	return w
}

// runNetHTTPRequest runs a request through a net/http ServeMux router.
func runNetHTTPRequest(t *testing.T, register func(routing.RouteGroup, *endpoint.Executor), method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	router := v2libNetHttp.NewRouter()
	exec := newTestExecutor()
	register(router, exec)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	serveHTTP(t, router, w, req)
	return w
}

// serveHTTP dispatches through the router's Native() handler.
func serveHTTP(t *testing.T, router routing.Router, w http.ResponseWriter, req *http.Request) {
	t.Helper()
	native := router.Native()
	h, ok := native.(http.Handler)
	if !ok {
		t.Fatalf("router.Native() does not implement http.Handler: %T", native)
	}
	h.ServeHTTP(w, req)
}

// capturingSink is a test telemetry sink that records events.
type capturingSink struct {
	events []telemetry.Event
}

func (s *capturingSink) Record(e telemetry.Event) {
	s.events = append(s.events, e)
}

// --- success and binding tests ---

func TestWrap_ChiSuccess(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := makeCreateUserEndpoint()
		if err := Register(g, exec, "POST", "/users", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com","age":30}`), "application/json")

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var resp createUserResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Name != "Alice" {
		t.Errorf("Name = %q", resp.Name)
	}
}

func TestWrap_NetHTTPSuccess(t *testing.T) {
	w := runNetHTTPRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := makeCreateUserEndpoint()
		if err := Register(g, exec, "POST", "/users", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/users", strings.NewReader(`{"name":"Alice","email":"alice@example.com","age":30}`), "application/json")

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	var resp createUserResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Name != "Alice" {
		t.Errorf("Name = %q", resp.Name)
	}
}

func TestWrap_JSONBindingAndValidation(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := makeCreateUserEndpoint()
		if err := Register(g, exec, "POST", "/users", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/users", strings.NewReader(`{"age":30}`), "application/json")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestWrap_UnsupportedContentType415(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := makeCreateUserEndpoint()
		if err := Register(g, exec, "POST", "/users", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/users", strings.NewReader(`{"name":"Alice","email":"a@x.com"}`), "text/plain")

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", w.Code)
	}
}

func TestWrap_OversizedBody413(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[createUserReq, createUserResp](
			func(ctx *request.Context, req createUserReq) (createUserResp, error) {
				return createUserResp{}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "createUser", Method: "POST", Pattern: "/users"}),
			endpoint.WithBindingPlan(binding.Plan{Mode: binding.ModeJSON, MaxBodyBytes: 10}),
		)
		if err := Register(g, exec, "POST", "/users", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/users", strings.NewReader(strings.Repeat("a", 100)), "application/json")

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

// --- error and panic tests ---

func TestWrap_HandlerErrorMappedProblem(t *testing.T) {
	notFoundErr := errors.New("user not found")
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		exec.ProblemMapper = response.NewMapperRegistry()
		exec.ProblemMapper.Register(
			func(err error) bool { return errors.Is(err, notFoundErr) },
			func(err error) *response.Problem {
				return response.NewProblemWithCode(http.StatusNotFound, "User Not Found", "USER_NOT_FOUND")
			},
		)
		ep := endpoint.New[problemReq, problemResp](
			func(ctx *request.Context, req problemReq) (problemResp, error) {
				if req.Mode == "notfound" {
					return problemResp{}, notFoundErr
				}
				return problemResp{Result: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "testProblem", Method: "POST", Pattern: "/test"}),
			endpoint.WithBindingPlan(binding.DefaultJSONPlan),
			endpoint.WithValidator(validation.New()),
		)
		if err := Register(g, exec, "POST", "/test", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/test", strings.NewReader(`{"mode":"notfound"}`), "application/json")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestWrap_HandlerPanicSanitized500(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[problemReq, problemResp](
			func(ctx *request.Context, req problemReq) (problemResp, error) {
				panic("boom")
			},
			endpoint.WithOperation(operation.Operation{ID: "testPanic", Method: "POST", Pattern: "/panic"}),
			endpoint.WithBindingPlan(binding.DefaultJSONPlan),
			endpoint.WithValidator(validation.New()),
		)
		if err := Register(g, exec, "POST", "/panic", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/panic", strings.NewReader(`{"mode":"ok"}`), "application/json")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

// --- path parameter tests ---

func TestWrap_ChiPathParam(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pathReq, pathResp](
			func(ctx *request.Context, req pathReq) (pathResp, error) {
				return pathResp{ID: req.ID}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "getUser", Method: "GET", Pattern: "/users/{id}"}),
			endpoint.WithBindingPlan(binding.DefaultPathPlan),
		)
		if err := Register(g, exec, "GET", "/users/{id}", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/users/42", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp pathResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "42" {
		t.Errorf("ID = %q, want 42", resp.ID)
	}
}

func TestWrap_NetHTTPPathParam(t *testing.T) {
	w := runNetHTTPRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pathReq, pathResp](
			func(ctx *request.Context, req pathReq) (pathResp, error) {
				return pathResp{ID: req.ID}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "getUser", Method: "GET", Pattern: "/users/{id}"}),
			endpoint.WithBindingPlan(binding.DefaultPathPlan),
		)
		if err := Register(g, exec, "GET", "/users/{id}", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/users/42", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp pathResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "42" {
		t.Errorf("ID = %q, want 42", resp.ID)
	}
}

// --- query parameter test ---

func TestWrap_QueryParam(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[queryReq, queryResp](
			func(ctx *request.Context, req queryReq) (queryResp, error) {
				return queryResp{Page: req.Page}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "listItems", Method: "GET", Pattern: "/items"}),
			endpoint.WithBindingPlan(binding.DefaultQueryPlan),
			endpoint.WithValidator(validation.New()),
		)
		if err := Register(g, exec, "GET", "/items", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/items?page=5", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp queryResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Page != 5 {
		t.Errorf("Page = %d, want 5", resp.Page)
	}
}

// --- dynamic status and headers ---

func TestWrap_DynamicStatusFromHandler(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				ctx.Response().SetStatus(http.StatusAccepted)
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
			endpoint.WithSuccessStatus(http.StatusOK),
		)
		if err := Register(g, exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (dynamic override)", w.Code)
	}
}

func TestWrap_HandlerResponseHeaders(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				ctx.Response().SetHeader("X-Custom", "value")
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		)
		if err := Register(g, exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if got := w.Header().Get("X-Custom"); got != "value" {
		t.Errorf("X-Custom = %q, want 'value'", got)
	}
}

func TestWrap_ConfiguredStatusWhenNotOverridden(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
			endpoint.WithSuccessStatus(http.StatusCreated),
		)
		if err := Register(g, exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (configured)", w.Code)
	}
}

// --- hook tests ---

func TestWrap_AlphaHookRunsOnce(t *testing.T) {
	var hookCount int32
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		)
		// Wrap with a middleware that adds a before-commit hook.
		hookMW := func(next routing.Handler) routing.Handler {
			return func(ctx *request.Context, transport routing.Transport) error {
				ctx.AddBeforeCommitHook(func() error {
					atomic.AddInt32(&hookCount, 1)
					return nil
				})
				return next(ctx, transport)
			}
		}
		if err := Register(g.With(hookMW), exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := atomic.LoadInt32(&hookCount); got != 1 {
		t.Errorf("alpha hook ran %d times, want 1", got)
	}
}

func TestWrap_StrictHookFailurePreventsSuccess(t *testing.T) {
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		)
		hookMW := func(next routing.Handler) routing.Handler {
			return func(ctx *request.Context, transport routing.Transport) error {
				ctx.AddBeforeCommitHook(func() error {
					return errors.New("strict hook failure")
				})
				return next(ctx, transport)
			}
		}
		if err := Register(g.With(hookMW), exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 status due to strict hook failure, got 200")
	}
}

// --- registration validation tests ---

func TestRegister_DuplicateOperationID(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep1 := makeCreateUserEndpoint()
	ep2 := makeCreateUserEndpoint()
	if err := Register(router, exec, "POST", "/users", ep1); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := Register(router, exec, "POST", "/users", ep2); err == nil {
		t.Fatal("expected duplicate ID error, got nil")
	}
}

func TestRegister_MethodMismatch(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	if err := Register(router, exec, "POST", "/ping", ep); err == nil {
		t.Fatal("expected method mismatch error, got nil")
	}
}

func TestRegister_PatternMismatch(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	if err := Register(router, exec, "GET", "/pong", ep); err == nil {
		t.Fatal("expected pattern mismatch error, got nil")
	}
}

func TestRegister_MissingOperationID(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{}, nil
		},
	)
	if err := Register(router, exec, "GET", "/ping", ep); err == nil {
		t.Fatal("expected missing operation ID error, got nil")
	}
}

// --- observability tests ---

func TestWrap_SuccessEmitsTelemetryEvent(t *testing.T) {
	sink := &capturingSink{}
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		exec.Telemetry = sink
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				return pingResp{Message: "ok"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
		)
		if err := Register(g, exec, "GET", "/ping", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/ping", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Verify a success telemetry event was emitted for the "ping" operation.
	foundSuccess := false
	for _, e := range sink.events {
		if e.Operation == "ping" && e.Type == telemetry.EventSuccess {
			foundSuccess = true
			if e.Status != http.StatusOK {
				t.Errorf("success event status = %d, want 200", e.Status)
			}
		}
	}
	if !foundSuccess {
		t.Fatal("expected success telemetry event for ping operation")
	}
}

func TestWrap_FailureEmitsTelemetryEvent(t *testing.T) {
	sink := &capturingSink{}
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		exec.Telemetry = sink
		ep := endpoint.New[problemReq, problemResp](
			func(ctx *request.Context, req problemReq) (problemResp, error) {
				return problemResp{}, errors.New("domain error")
			},
			endpoint.WithOperation(operation.Operation{ID: "testFail", Method: "POST", Pattern: "/fail"}),
			endpoint.WithBindingPlan(binding.DefaultJSONPlan),
		)
		if err := Register(g, exec, "POST", "/fail", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "POST", "/fail", strings.NewReader(`{"mode":"ok"}`), "application/json")

	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 status")
	}
	// Verify a failure telemetry event was emitted for the "testFail" operation.
	foundFailure := false
	for _, e := range sink.events {
		if e.Operation == "testFail" && e.Type == telemetry.EventFailure {
			foundFailure = true
			if e.Err == nil {
				t.Error("expected error in failure telemetry event")
			}
		}
	}
	if !foundFailure {
		t.Fatal("expected failure telemetry event for testFail operation")
	}
}

func TestWrap_TelemetryEventsHaveNoResponseBody(t *testing.T) {
	sink := &capturingSink{}
	w := runChiRequest(t, func(g routing.RouteGroup, exec *endpoint.Executor) {
		exec.Telemetry = sink
		ep := endpoint.New[pingReq, pingResp](
			func(ctx *request.Context, req pingReq) (pingResp, error) {
				return pingResp{Message: "topsecret"}, nil
			},
			endpoint.WithOperation(operation.Operation{ID: "secret", Method: "GET", Pattern: "/secret"}),
		)
		if err := Register(g, exec, "GET", "/secret", ep); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}, "GET", "/secret", nil, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// The HTTP response body should contain the raw response.
	body := w.Body.String()
	if !strings.Contains(body, "topsecret") {
		t.Fatalf("expected raw response in HTTP body, got: %s", body)
	}
	// Telemetry events must never include response bodies. The Event
	// struct has no body field, so we verify that no event's attrs
	// contain the secret.
	for _, e := range sink.events {
		for _, attr := range e.Attrs {
			rendered := attr.Value.String()
			if strings.Contains(rendered, "topsecret") {
				t.Errorf("telemetry event attr contains response body: %s", rendered)
			}
		}
	}
}

// --- concurrency test ---

func TestWrap_ConcurrentRequestsNoRace(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep := endpoint.New[pingReq, pingResp](
		func(ctx *request.Context, req pingReq) (pingResp, error) {
			return pingResp{Message: "ok"}, nil
		},
		endpoint.WithOperation(operation.Operation{ID: "ping", Method: "GET", Pattern: "/ping"}),
	)
	if err := Register(router, exec, "GET", "/ping", ep); err != nil {
		t.Fatalf("Register: %v", err)
	}

	native := router.Native().(http.Handler)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/ping", nil)
			native.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", w.Code)
			}
		}()
	}
	wg.Wait()
}

// --- committed error response prevents double-write ---

func TestWrap_CommittedProblemReturnsNil(t *testing.T) {
	router := v2libChi.NewRouter()
	exec := newTestExecutor()
	ep := endpoint.New[problemReq, problemResp](
		func(ctx *request.Context, req problemReq) (problemResp, error) {
			return problemResp{}, errors.New("domain error")
		},
		endpoint.WithOperation(operation.Operation{ID: "testFail", Method: "POST", Pattern: "/fail"}),
		endpoint.WithBindingPlan(binding.DefaultJSONPlan),
	)
	if err := Register(router, exec, "POST", "/fail", ep); err != nil {
		t.Fatalf("Register: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/fail", strings.NewReader(`{"mode":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	native := router.Native().(http.Handler)
	native.ServeHTTP(w, req)

	// The problem response should be written exactly once with the
	// mapped status (500 for unknown error).
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// The body should be a single problem JSON, not duplicated.
	body := w.Body.String()
	if strings.Count(body, `"title"`) != 1 {
		t.Errorf("expected exactly one problem in body, got: %s", body)
	}
}
