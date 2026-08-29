package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/libRequest"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
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

// TestAddLog_SuccessPathEmitsTitleReq verifies that a successful handler
// emits the mandatory <title>-req AddLog entry containing the parsed
// response, per the enterprise AddLog convention.
func TestAddLog_SuccessPathEmitsTitleReq(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, TestResp]("addlog-title-req", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, TestResp]) (TestResp, error) {
			capturedParser = trx.V2.Parser
			return TestResp{Status: "ok"}, nil
		},
	).WithPath("/title-req")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/title-req", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/title-req", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	key := "LOG_ARRAY_addlog-title-req-req"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected <title>-req AddLog entry for %q, got nil", key)
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least one attr in <title>-req array")
	}
}

// maskedResp implements slog.LogValuer to test the AddLog masking
// projection. The returned HTTP response is the raw struct; AddLog
// receives the LogValue projection.
type maskedResp struct {
	Status   string `json:"status"`
	Secret   string `json:"secret"`
	loggedAs slog.Value
}

func (m maskedResp) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("status", m.Status),
		slog.String("secret", "<masked>"),
	)
}

// TestAddLog_SuccessPathLogValuerMasking verifies that when the response
// implements slog.LogValuer, the <title>-req AddLog entry receives the
// LogValue projection rather than the raw response, and the returned HTTP
// response body is unaffected.
func TestAddLog_SuccessPathLogValuerMasking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, maskedResp]("addlog-mask", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, maskedResp]) (maskedResp, error) {
			capturedParser = trx.V2.Parser
			return maskedResp{Status: "ok", Secret: "topsecret"}, nil
		},
	).WithPath("/mask")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/mask", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mask", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// The HTTP response body must contain the raw secret (unmodified).
	body := w.Body.String()
	if !strings.Contains(body, "topsecret") {
		t.Fatalf("expected raw secret in HTTP response body, got: %s", body)
	}
	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	// The <title>-req AddLog entry should contain the LogValue projection.
	key := "LOG_ARRAY_addlog-mask-req"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected <title>-req AddLog entry for %q, got nil", key)
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least one attr in <title>-req array")
	}
	// The response attr should be a slog.Value (the LogValue projection),
	// not the raw struct. Verify it does not contain "topsecret" when
	// rendered. The attr value kind will be KindGroup from LogValue.
	respAttr := arr[len(arr)-1]
	if respAttr.Key != "response" {
		// Find the response attr.
		found := false
		for _, a := range arr {
			if a.Key == "response" {
				respAttr = a
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected 'response' attr in <title>-req array, got: %+v", arr)
		}
	}
	// The LogValue projection should not contain the raw secret string.
	rendered := respAttr.Value.String()
	if strings.Contains(rendered, "topsecret") {
		t.Fatalf("expected masked projection without raw secret, got: %s", rendered)
	}
}

// panickingLogValuer implements slog.LogValuer but panics in LogValue.
type panickingLogValuer struct {
	Status string `json:"status"`
}

func (panickingLogValuer) LogValue() slog.Value {
	panic("logvalue boom")
}

// TestAddLog_SuccessPathLogValuerPanic verifies that a panic in LogValue
// is recovered and the raw response is never logged as a fallback.
func TestAddLog_SuccessPathLogValuerPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router := v2libGin.NewRouter(engine)
	respHandler := testRespHandler()

	var capturedParser v2wf.RequestParser
	e := NewEndpoint[struct{}, panickingLogValuer]("addlog-panic-lv", libRequest.NoBinding,
		func(req *struct{}, trx *HandlerRequest[struct{}, panickingLogValuer]) (panickingLogValuer, error) {
			capturedParser = trx.V2.Parser
			return panickingLogValuer{Status: "ok"}, nil
		},
	).WithPath("/panic-lv")
	if err := RegisterEndpoint(router, nil, respHandler, "GET", "/panic-lv", e); err != nil {
		t.Fatalf("RegisterEndpoint: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic-lv", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if capturedParser == nil {
		t.Fatal("expected parser to be captured")
	}
	key := "LOG_ARRAY_addlog-panic-lv-req"
	v := capturedParser.GetLocal(key)
	if v == nil {
		t.Fatalf("expected <title>-req AddLog entry for %q, got nil", key)
	}
	arr, ok := v.([]slog.Attr)
	if !ok {
		t.Fatalf("expected []slog.Attr, got %T", v)
	}
	// The masking-failure placeholder should be present, not the raw struct.
	found := false
	for _, a := range arr {
		if a.Key == "response" {
			rendered := a.Value.String()
			if strings.Contains(rendered, "Status") || strings.Contains(rendered, "ok") {
				t.Fatalf("expected masked panic placeholder, got raw value: %s", rendered)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'response' attr in <title>-req array, got: %+v", arr)
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
	).WithPath("/init-fail").WithInitializer(func(trx *HandlerRequest[struct{}, TestResp]) error {
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
