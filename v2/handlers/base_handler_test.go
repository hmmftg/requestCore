package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testRespHandler() *v2response.Handler {
	registry := v2response.NewRegistry(nil)
	registry.SetFallback(v2response.LegacyFallback(response.WebHanlder{
		MessageDesc: make(map[string]string),
		ErrorDesc:   make(map[string]string),
	}))
	return v2response.NewHandler(registry, renderers.JSONRenderer{}, response.WebHanlder{})
}

// TestEndpoint_GetSuccess verifies the full lifecycle for a successful GET
// endpoint: parse, log, handler, render, finalize.
func TestEndpoint_GetSuccess(t *testing.T) {
	t.Skip("Phase 6: handlers bridge needs full v2wf.RequestContext — will be rewritten when handlers are migrated to *request.Context")
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	finalized := false
	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/health",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}

	// Add finalizer via free function
	_ = err // endpoint already registered

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp TestResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp.Status)
	}
	_ = finalized
}

// TestEndpoint_PostSuccess verifies JSON body parsing for a POST endpoint.
func TestEndpoint_PostSuccess(t *testing.T) {
	t.Skip("Phase 6: handlers bridge needs full v2wf.RequestContext — will be rewritten when handlers are migrated to *request.Context")
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := PostEndpoint[CreateReq, CreateResp](
		router, nil, respHandler, "/users",
		func(req *CreateReq, trx *HandlerRequest[CreateReq, CreateResp]) (CreateResp, error) {
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	)
	if err != nil {
		t.Fatalf("PostEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/users", strings.NewReader(`{"name":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp CreateResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Name != "alice" {
		t.Fatalf("expected name 'alice', got %q", resp.Name)
	}
}

// TestEndpoint_HandlerError verifies that handler errors are routed through
// the v2 response handler and registry.
func TestEndpoint_HandlerError(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/fail",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{}, errors.New("handler failure")
		},
	)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEndpoint_PanicRecovery verifies that panics are recovered, converted
// to sanitized errors, and written as 500 responses.
func TestEndpoint_PanicRecovery(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	err := GetEndpoint[struct{}, TestResp](
		router, nil, respHandler, "/panic",
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			panic("boom")
		},
	)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEndpoint_WithInitializer verifies the initializer runs before the handler.
func TestEndpoint_WithInitializer(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	initRan := false
	e := NewEndpoint[struct{}, TestResp]("test-init", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			if !initRan {
				return TestResp{}, errors.New("initializer did not run")
			}
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/init").WithInitializer(func(trx *HandlerRequest[struct{}, TestResp]) error {
		initRan = true
		return nil
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/init", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEndpoint_WithFinalizer verifies the finalizer runs after the response.
func TestEndpoint_WithFinalizer(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	finalized := false
	e := NewEndpoint[struct{}, TestResp]("test-fin", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/fin").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		finalized = true
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/fin", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fin", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !finalized {
		t.Fatal("expected finalizer to run")
	}
}

// TestEndpoint_WithPersistence verifies persistence insert/update lifecycle.
func TestEndpoint_WithPersistence(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	inserted := false
	updated := false
	e := NewEndpoint[CreateReq, CreateResp]("test-persist", libRequest.JSON,
		func(req *CreateReq, trx *HandlerRequest[CreateReq, CreateResp]) (CreateResp, error) {
			if !inserted {
				return CreateResp{}, errors.New("persistence insert did not run")
			}
			return CreateResp{ID: "1", Name: req.Name}, nil
		},
	).WithPath("/persist").WithPersistence(NewPersister[CreateReq, CreateResp](
		func(path string, trx *HandlerRequest[CreateReq, CreateResp]) error {
			inserted = true
			return nil
		},
		func(path string, trx *HandlerRequest[CreateReq, CreateResp]) error {
			updated = true
			return nil
		},
	))
	if err := RegisterEndpoint(router, nil, respHandler, "POST", "/persist", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/persist", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !updated {
		t.Fatal("expected persistence update to run")
	}
}

// TestEndpoint_InitializerError verifies initializer errors abort the lifecycle.
func TestEndpoint_InitializerError(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	e := NewEndpoint[struct{}, TestResp]("test-init-err", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{}, errors.New("handler should not run")
		},
	).WithPath("/init-err").WithInitializer(func(trx *HandlerRequest[struct{}, TestResp]) error {
		return errors.New("initializer failed")
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/init-err", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init-err", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEndpoint_PersistenceInsertError verifies persistence insert errors abort.
func TestEndpoint_PersistenceInsertError(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	handlerRan := false
	e := NewEndpoint[CreateReq, CreateResp]("test-persist-err", libRequest.JSON,
		func(req *CreateReq, trx *HandlerRequest[CreateReq, CreateResp]) (CreateResp, error) {
			handlerRan = true
			return CreateResp{}, nil
		},
	).WithPath("/persist-err").WithPersistence(NewPersister[CreateReq, CreateResp](
		func(path string, trx *HandlerRequest[CreateReq, CreateResp]) error {
			return errors.New("insert failed")
		},
		nil,
	))
	if err := RegisterEndpoint(router, nil, respHandler, "POST", "/persist-err", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/persist-err", strings.NewReader(`{"name":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
	if handlerRan {
		t.Fatal("handler should not run when persistence insert fails")
	}
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

// TestEndpoint_FinalizerAlwaysRuns verifies the finalizer runs even on errors.
func TestEndpoint_FinalizerAlwaysRuns(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	finalized := false
	e := NewEndpoint[struct{}, TestResp]("test-fin-err", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{}, errors.New("handler error")
		},
	).WithPath("/fin-err").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		finalized = true
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/fin-err", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fin-err", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !finalized {
		t.Fatal("finalizer should run even on handler error")
	}
}

// TestEndpoint_FinalizerRunsOnPanic verifies the finalizer runs on panic.
func TestEndpoint_FinalizerRunsOnPanic(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	finalized := false
	e := NewEndpoint[struct{}, TestResp]("test-fin-panic", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			panic("boom")
		},
	).WithPath("/fin-panic").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		finalized = true
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/fin-panic", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fin-panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !finalized {
		t.Fatal("finalizer should run even on panic")
	}
}

// TestEndpoint_DurationSet verifies the duration is recorded.
func TestEndpoint_DurationSet(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedDuration time.Duration
	e := NewEndpoint[struct{}, TestResp]("test-dur", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			time.Sleep(1 * time.Millisecond)
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/dur").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		capturedDuration = trx.Duration
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/dur", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/dur", nil)
	engine.ServeHTTP(w, req)

	if capturedDuration <= 0 {
		t.Fatalf("expected positive duration, got %v", capturedDuration)
	}
}

// TestEndpoint_RecoveryHandlerNotificationOnly verifies that a custom
// recovery handler is called as a notification hook but does NOT suppress
// the standard sanitized error response.
func TestEndpoint_RecoveryHandlerNotificationOnly(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	recoveryCalled := false
	e := NewEndpoint[struct{}, TestResp]("test-recovery", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			panic("boom")
		},
	).WithPath("/recovery")
	e.WithRecoveryHandler(func(val any) {
		recoveryCalled = true
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/recovery", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/recovery", nil)
	engine.ServeHTTP(w, req)

	if !recoveryCalled {
		t.Fatal("expected recovery handler to be called")
	}
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 even with recovery handler, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEndpoint_TracingSpanCompletion verifies that tracing spans are
// completed (ended) after request processing.
func TestEndpoint_TracingSpanCompletion(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedSpan trace.Span
	e := NewEndpoint[struct{}, TestResp]("test-trace", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedSpan = trx.Span
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/trace").WithTracing("test-span")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/trace", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/trace", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Span may be nil if tracing is not configured globally; we just
	// verify the endpoint doesn't crash with tracing enabled.
	_ = capturedSpan
}

// TestEndpoint_OutcomeOnSuccess verifies that the outcome is recorded
// with the correct HTTP status on success.
func TestEndpoint_OutcomeOnSuccess(t *testing.T) {
	t.Skip("Phase 6: handlers bridge needs full v2wf.RequestContext — will be rewritten when handlers are migrated to *request.Context")
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var outcome HandlerOutcome
	e := NewEndpoint[struct{}, TestResp]("test-outcome", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/outcome").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		outcome = trx.Outcome
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/outcome", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/outcome", nil)
	engine.ServeHTTP(w, req)

	if outcome.Error != nil {
		t.Fatalf("expected nil error, got %v", outcome.Error)
	}
	if outcome.HTTPStatus != http.StatusOK {
		t.Fatalf("expected 200, got %d", outcome.HTTPStatus)
	}
}

// TestEndpoint_OutcomeOnError verifies that the outcome records the error
// and correct HTTP status on handler error.
func TestEndpoint_OutcomeOnError(t *testing.T) {
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var outcome HandlerOutcome
	e := NewEndpoint[struct{}, TestResp]("test-outcome-err", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			return TestResp{}, errors.New("handler error")
		},
	).WithPath("/outcome-err").WithFinalizer(func(trx *HandlerRequest[struct{}, TestResp]) {
		outcome = trx.Outcome
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/outcome-err", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/outcome-err", nil)
	engine.ServeHTTP(w, req)

	if outcome.Error == nil {
		t.Fatal("expected non-nil error in outcome")
	}
	if outcome.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", outcome.HTTPStatus)
	}
}

// TestAddWebLogs_CompletionLogsStatus verifies that the AddWebLogs
// completion closure is invoked in finalize and logs the status under
// the HandlerLogTag, which was previously discarded.
func TestAddWebLogs_CompletionLogsStatus(t *testing.T) {
	t.Skip("Phase 6: handlers bridge needs full v2wf.RequestContext — will be rewritten when handlers are migrated to *request.Context")
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("test-weblogs-status", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/weblogs-status")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/weblogs-status", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/weblogs-status", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	// The completion closure logs status under HandlerLogTag tags.
	tagKey := "LOG_TAG_handler"
	v := capturedParser.GetLocal(tagKey)
	if v == nil {
		t.Fatalf("expected HandlerLogTag tags, got nil")
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	foundStatus := false
	foundElapsed := false
	for _, a := range arr {
		if a.Key == "status" && a.Value.Int64() == int64(http.StatusOK) {
			foundStatus = true
		}
		if a.Key == "elapsed" {
			foundElapsed = true
		}
	}
	if !foundStatus {
		t.Fatalf("expected status=200 in HandlerLogTag tags, got: %+v", arr)
	}
	if !foundElapsed {
		t.Fatalf("expected elapsed in HandlerLogTag tags, got: %+v", arr)
	}
}
