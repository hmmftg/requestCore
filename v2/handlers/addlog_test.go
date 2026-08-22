package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"log/slog"

	"github.com/hmmftg/requestCore/libRequest"
	"github.com/hmmftg/requestCore/response"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	"github.com/hmmftg/requestCore/v2/renderers"
	v2response "github.com/hmmftg/requestCore/v2/response"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// TestAddLog_FailurePathEmitsReqFailed verifies that when a handler
// returns an error, the lifecycle emits a mandatory AddLog entry with
// the "<title>-req-failed" key into the Splunk transaction pipeline.
func TestAddLog_FailurePathEmitsReqFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("addlog-fail", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			return TestResp{}, errors.New("handler failure")
		},
	).WithPath("/fail")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/fail", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	engine.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 status, got 200")
	}

	// Verify the AddLog failure entry was emitted with the
	// "addlog-fail-req-failed" key.
	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	key := "LOG_ARRAY_addlog-fail-req-failed"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected AddLog entry for %q, got nil", key)
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least one log attr in failure array")
	}
}

// TestAddLog_PanicPathEmitsReqFailed verifies that when a handler
// panics, the lifecycle emits a mandatory AddLog entry with the
// "<title>-req-failed" key.
func TestAddLog_PanicPathEmitsReqFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("addlog-panic", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			panic("boom")
		},
	).WithPath("/panic")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/panic", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	key := "LOG_ARRAY_addlog-panic-req-failed"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected AddLog entry for %q, got nil", key)
	}
}

// TestAddLog_SuccessPathEmitsHandlerLog verifies that a successful
// handler emits AddLog entries under the HandlerLogTag.
func TestAddLog_SuccessPathEmitsHandlerLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("addlog-ok", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/ok")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/ok", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	// The handler log tag array should contain entries.
	key := "LOG_ARRAY_handler"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected AddLog entries for %q, got nil", key)
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least one handler log attr")
	}
}

// TestAddLog_InitializerFailureEmitsReqFailed verifies that an
// initializer failure emits the req-failed AddLog entry.
func TestAddLog_InitializerFailureEmitsReqFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("addlog-init-fail", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			return TestResp{}, errors.New("handler should not run")
		},
	).WithPath("/init-fail")
	WithInitializer[struct{}, TestResp](e, func(trx *HandlerRequest[struct{}, TestResp]) error {
		capturedParser = trx.V2.Parser
		return errors.New("initializer failed")
	})
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/init-fail", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/init-fail", nil)
	engine.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Fatalf("expected non-200 status, got 200")
	}

	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	key := "LOG_ARRAY_addlog-init-fail-req-failed"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected AddLog entry for %q, got nil", key)
	}
}

// Ensure the test file compiles even if some imports are unused in
// future edits.
var _ = strings.NewReader
var _ = renderers.JSONRenderer{}
var _ = response.WebHanlder{}
var _ = v2response.NewHandler
