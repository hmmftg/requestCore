package handlers_test

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/handlers"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/prometheus/client_golang/prometheus"
	"gotest.tools/v3/assert"
)

type optsTestResponse struct {
	Data   []optsTestData `json:"data"`
	Status string         `json:"status"`
	Count  int            `json:"count"`
}

type optsTestData struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s optsTestResponse) SetStatus(a int)                {}
func (s optsTestResponse) SetHeaders(a map[string]string) {}

func setupOptsTest(t *testing.T) (*libCallApi.FakeAPIServer, *libCallApi.RemoteCallParamData[any, optsTestResponse]) {
	t.Helper()
	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
	fakeServer := libCallApi.NewFakeAPIServer()
	t.Cleanup(fakeServer.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: fakeServer.URL() + "/api"},
		Method: "GET",
		Path:   "test1",
	}
	return fakeServer, param
}

func TestCallApiJSONWithOpts_Success(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	var onCompleteCalled bool
	var capturedInfo handlers.ApiCallInfo

	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-success",
			OnComplete: func(info handlers.ApiCallInfo) {
				onCompleteCalled = true
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, resp.Status, "success")
	assert.Equal(t, len(resp.Data), 2)
	assert.Assert(t, onCompleteCalled, "OnComplete should be called")
	assert.Equal(t, capturedInfo.StatusCode, http.StatusOK)
	assert.Assert(t, capturedInfo.Error == nil, "error should be nil on success")
	assert.Equal(t, capturedInfo.Method, "GET")
	assert.Equal(t, capturedInfo.Endpoint, "test1")
	assert.Equal(t, capturedInfo.URL, fakeServer.URL()+"/api/test1")
	assert.Assert(t, capturedInfo.ResponseBody == nil, "ResponseBody should be nil on success")

	// Verify webFramework.AddLog was called: request log + response log = 2 entries
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	assert.Assert(t, logs != nil, "AddLog entries should exist")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok, "logs should be []slog.Attr")
	assert.Equal(t, len(logArr), 2, "success path should have 2 log entries (req + resp)")
	assert.Equal(t, logArr[0].Key, "test-success")
	assert.Equal(t, logArr[1].Key, "test-success-resp")
}

func TestCallApiJSONWithOpts_Error(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "nonexistent-path-for-error"
	param.Api.Domain = "http://localhost:1" // unreachable port to force error
	w := libContext.InitContextNoAuditTrail(t)

	var onCompleteCalled bool
	var capturedInfo handlers.ApiCallInfo

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-error",
			OnComplete: func(info handlers.ApiCallInfo) {
				onCompleteCalled = true
				capturedInfo = info
			},
		},
	)

	assert.Assert(t, err != nil, "should return error for unreachable server")
	assert.Assert(t, onCompleteCalled, "OnComplete should be called on error")
	assert.Assert(t, capturedInfo.Error != nil, "error should be set in ApiCallInfo")
	assert.Equal(t, capturedInfo.StatusCode, 0, "statusCode should be 0 for connection error")

	// Verify webFramework.AddLog: request log + error log = 2 entries
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	assert.Assert(t, logs != nil, "AddLog entries should exist")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok, "logs should be []slog.Attr")
	assert.Equal(t, len(logArr), 2, "error path should have 2 log entries (req + error)")
	assert.Equal(t, logArr[0].Key, "test-error")
	assert.Equal(t, logArr[1].Key, "test-error-error")
}

func TestCallApiJSONWithOpts_Timeout(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:  "test-timeout",
			Timeout: 1 * time.Nanosecond, // impossibly small timeout
		},
	)

	assert.Assert(t, err != nil, "should return timeout error")
	errMsg := err.Error()
	assert.Assert(t, strings.Contains(errMsg, "elapsed"), "timeout error should contain 'elapsed', got: %s", errMsg)
	assert.Assert(t, strings.Contains(errMsg, "timeout"), "timeout error should contain 'timeout', got: %s", errMsg)
	assert.Assert(t, strings.Contains(errMsg, "server-side timeout exceeded"), "timeout error should contain 'server-side timeout exceeded', got: %s", errMsg)
	// Timeout error should contain the API domain, not just the name
	assert.Assert(t, strings.Contains(errMsg, fakeServer.URL()+"/api"), "timeout error should contain API domain, got: %s", errMsg)

	// Verify webFramework.AddLog: request log + resp log + timeout error log = 3 entries
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	assert.Assert(t, logs != nil, "AddLog entries should exist")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok, "logs should be []slog.Attr")
	assert.Equal(t, len(logArr), 3, "timeout path should have 3 log entries (req + resp + timeout-error)")
	assert.Equal(t, logArr[0].Key, "test-timeout")
	assert.Equal(t, logArr[1].Key, "test-timeout-resp")
	assert.Equal(t, logArr[2].Key, "test-timeout-error")
}

func TestCallApiJSONWithOpts_HTTP403(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-403",
		},
	)

	assert.Assert(t, err != nil, "should return error for 403")
	assert.Equal(t, libCallApi.IsForbidden(err), true)

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "err should be a RemoteCallError")
	assert.Equal(t, rce.Status, http.StatusForbidden)
	assert.Assert(t, len(rce.Body) > 0, "body should be preserved")
}

func TestCallApiJSONWithOpts_HTTP401(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "unauthorized"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-401",
		},
	)

	assert.Assert(t, err != nil, "should return error for 401")
	assert.Equal(t, libCallApi.IsForbidden(err), true)

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "err should be a RemoteCallError")
	assert.Equal(t, rce.Status, http.StatusUnauthorized)
}

func TestCallApiJSONWithOpts_HTTP500(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "server-error"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-500",
		},
	)

	assert.Assert(t, err != nil, "should return error for 500")
	assert.Equal(t, libCallApi.IsForbidden(err), false)

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "err should be a RemoteCallError")
	assert.Equal(t, rce.Status, http.StatusInternalServerError)
}

func TestCallApiJSONWithOpts_OnCompleteOnError(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-oncomplete-error",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, capturedInfo.StatusCode, http.StatusForbidden)
	assert.Assert(t, capturedInfo.Error != nil)
	assert.Equal(t, capturedInfo.URL, fakeServer.URL()+"/api/forbidden")
	assert.Assert(t, len(capturedInfo.ResponseBody) > 0, "ResponseBody should contain raw error body")
	assert.Assert(t, strings.Contains(string(capturedInfo.ResponseBody), "forbidden"), "ResponseBody should contain 'forbidden'")
}

func TestCallApiJSONWithOpts_NoOnComplete(t *testing.T) {
	_, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	// Should not panic when OnComplete is nil
	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-no-callback",
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, resp.Status, "success")
}

func TestCallApiJSONWithOpts_201Created(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "created"
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-201",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err, "201 should not be an error")
	assert.Equal(t, capturedInfo.StatusCode, http.StatusCreated)
	_ = resp
}

func TestCallApiJSONWithOpts_204NoContent(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "no-content"
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-204",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err, "204 should not be an error")
	assert.Equal(t, capturedInfo.StatusCode, http.StatusNoContent)
}

func TestCallApiJSONWithOpts_MetricsRecorded(t *testing.T) {
	_, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-metrics",
		},
	)

	assert.NilError(t, err)

	// Verify Prometheus metrics were recorded by collecting from the default registry
	mfs, err := prometheus.DefaultGatherer.Gather()
	assert.NilError(t, err)

	var foundCounter, foundHistogram bool
	for _, mf := range mfs {
		if mf.GetName() == "http_client_calls_total" {
			foundCounter = true
		}
		if mf.GetName() == "http_client_call_duration_seconds" {
			foundHistogram = true
		}
	}
	assert.Assert(t, foundCounter, "http_client_calls_total metric should be registered")
	assert.Assert(t, foundHistogram, "http_client_call_duration_seconds metric should be registered")
}
