package handlers_test

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/handlers"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"github.com/hmmftg/requestCore/libError"
	"github.com/hmmftg/requestCore/libRetry"
	"github.com/hmmftg/requestCore/libTracing"
	"github.com/hmmftg/requestCore/webFramework"
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
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:  "test-timeout",
			Timeout: 10 * time.Millisecond, // slow endpoint sleeps 50ms
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

	recorder := &fakeMetricsRecorder{}
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-metrics",
			MetricsRecorder: recorder,
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, len(recorder.calls), 1, "recorder should be called once")
	assert.Equal(t, recorder.calls[0].method, "GET")
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusOK)
	assert.Equal(t, recorder.calls[0].outcome, "success")
	assert.Assert(t, recorder.calls[0].duration >= 0, "duration should be non-negative")
}

func TestCallApiJSONWithOpts_MetricsFailureOn403(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	recorder := &fakeMetricsRecorder{}
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-metrics-403",
			MetricsRecorder: recorder,
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, len(recorder.calls), 1)
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusForbidden)
	assert.Equal(t, recorder.calls[0].outcome, "failure")
}

func TestCallApiJSONWithOpts_MetricsTimeoutOutcome(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	recorder := &fakeMetricsRecorder{}
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-metrics-timeout",
			Timeout:         10 * time.Millisecond,
			MetricsRecorder: recorder,
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, len(recorder.calls), 1)
	assert.Equal(t, recorder.calls[0].outcome, "timeout")
	assert.Assert(t, recorder.calls[0].statusCode >= 200 && recorder.calls[0].statusCode < 300,
		"timeout should still record actual HTTP status, got: %d", recorder.calls[0].statusCode)
}

func TestCallApiJSONWithOpts_CustomBuilderNon2xx(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	builderCalled := false
	param.Builder = func(stat int, rawResp []byte, headers map[string]string) (*optsTestResponse, error) {
		builderCalled = true
		return &optsTestResponse{}, nil
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-custom-builder-403",
		},
	)

	assert.Assert(t, err != nil, "should return error for 403")
	assert.Assert(t, !builderCalled, "custom builder should NOT be called for non-2xx response")

	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "err should be a RemoteCallError even with custom builder")
	assert.Equal(t, rce.Status, http.StatusForbidden)
	assert.Equal(t, libCallApi.IsForbidden(err), true)
}

func TestCallApiJSONWithOpts_TransactionLogger(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-tx-logger",
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, resp.Status, "success")
	assert.Equal(t, len(logger.calls), 1, "TransactionLogger should be called once on success")
	assert.Equal(t, logger.calls[0].StatusCode, http.StatusOK)
	assert.Equal(t, logger.calls[0].Method, "GET")
	assert.Equal(t, logger.calls[0].URL, fakeServer.URL()+"/api/test1")
	assert.Assert(t, logger.calls[0].Error == nil, "error should be nil on success")
	assert.Assert(t, logger.calls[0].Duration >= 0, "duration should be non-negative")
}

func TestCallApiJSONWithOpts_TransactionLoggerOnError(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-tx-logger-error",
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, len(logger.calls), 1, "TransactionLogger should be called on error")
	assert.Equal(t, logger.calls[0].StatusCode, http.StatusForbidden)
	assert.Assert(t, logger.calls[0].Error != nil, "error should be set in transaction info")
	assert.Assert(t, len(logger.calls[0].ResponseBody) > 0, "ResponseBody should contain raw error body")
}

func TestCallApiJSONWithOpts_TransactionLoggerOnTimeout(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:  "test-tx-logger-timeout",
			Timeout: 10 * time.Millisecond,
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, len(logger.calls), 1, "TransactionLogger should be called on timeout")
	assert.Assert(t, logger.calls[0].Error != nil, "error should be set on timeout")
	assert.Assert(t, strings.Contains(logger.calls[0].Error.Error(), "timeout"), "timeout error should contain 'timeout'")
}

func TestCallApiJSONWithOpts_TransactionLoggerAbsent(t *testing.T) {
	_, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	// No TransactionLogger registered — should not panic
	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-no-tx-logger",
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, resp.Status, "success")
}

func TestCallApiJSONWithOpts_QueryInURL(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Query = "?page=1&limit=10"
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-query-url",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, capturedInfo.URL, fakeServer.URL()+"/api/test1?page=1&limit=10")
}

func TestCallApiJSONWithOpts_QueryStackInURL(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Query = ""
	param.QueryStack = &[]string{"?page=2"}
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-querystack-url",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, capturedInfo.URL, fakeServer.URL()+"/api/test1?page=2",
		"URL should reflect QueryStack-selected query, got: %s", capturedInfo.URL)
}

func TestBuildRequestURL(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		path   string
		query  string
		want   string
	}{
		{"basic", "http://example.com", "api/test", "", "http://example.com/api/test"},
		{"trailing slash domain", "http://example.com/", "api/test", "", "http://example.com//api/test"},
		{"leading slash path", "http://example.com", "/api/test", "", "http://example.com//api/test"},
		{"query with ?", "http://example.com", "api/test", "?page=1", "http://example.com/api/test?page=1"},
		{"query without ?", "http://example.com", "api/test", "page=1", "http://example.com/api/testpage=1"},
		{"empty query", "http://example.com", "api/test", "", "http://example.com/api/test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handlers.BuildRequestURL(tt.domain, tt.path, tt.query)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestInitHTTPClientMetricsNoPanic(t *testing.T) {
	// Calling InitHTTPClientMetrics multiple times should not panic
	libTracing.InitHTTPClientMetrics()
	libTracing.InitHTTPClientMetrics()
	libTracing.InitHTTPClientMetrics()
}

func TestCallApiJSONWithOpts_Malformed2xxPreservesStatus(t *testing.T) {
	_, param := setupOptsTest(t)
	// Add a malformed endpoint to the fake server
	w := libContext.InitContextNoAuditTrail(t)

	recorder := &fakeMetricsRecorder{}
	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	// Use a custom builder that returns a parse error for 200
	param.Builder = func(stat int, rawResp []byte, headers map[string]string) (*optsTestResponse, error) {
		return nil, fmt.Errorf("custom parse error")
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-malformed-200",
			MetricsRecorder: recorder,
		},
	)

	assert.Assert(t, err != nil, "should return error for custom builder failure")
	// StatusCode should be 200, not 0 — actualStatus was captured
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusOK,
		"metrics should record actual HTTP status 200, not 0")
	assert.Equal(t, recorder.calls[0].outcome, "failure")
	assert.Equal(t, logger.calls[0].StatusCode, http.StatusOK,
		"TransactionInfo should preserve actual HTTP status 200")
}

func TestCallApiJSONWithOpts_URLMatchesActualRequest(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Query = "?page=1&limit=10"
	w := libContext.InitContextNoAuditTrail(t)

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-url-match",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	// The transaction URL should match what PrepareCall actually sends:
	// Domain + "/" + Path + Query
	expectedURL := fakeServer.URL() + "/api/test1?page=1&limit=10"
	assert.Equal(t, capturedInfo.URL, expectedURL,
		"transaction URL should match actual request URL")
}

// fakeMetricsRecorder captures Record calls for test verification.
type fakeMetricsRecorder struct {
	calls []fakeMetricsCall
}

type fakeMetricsCall struct {
	api        string
	method     string
	statusCode int
	duration   time.Duration
	outcome    string
}

func (r *fakeMetricsRecorder) Record(api, method string, statusCode int, duration time.Duration, outcome string) {
	r.calls = append(r.calls, fakeMetricsCall{api, method, statusCode, duration, outcome})
}

// fakeTransactionLogger captures LogTransaction calls for test verification.
type fakeTransactionLogger struct {
	calls []webFramework.TransactionInfo
}

func (l *fakeTransactionLogger) LogTransaction(info webFramework.TransactionInfo) {
	l.calls = append(l.calls, info)
}

func TestCallApiJSONWithOpts_ConfigurableLogKeys(t *testing.T) {
	_, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-default",
			LogKeys: handlers.CallApiLogKeys{
				Request:  "svc-test-req",
				Response: "svc-test-resp",
				Failure:  "svc-test-fail",
			},
		},
	)

	assert.NilError(t, err)
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	assert.Assert(t, logs != nil, "AddLog entries should exist")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok, "logs should be []slog.Attr")
	assert.Equal(t, len(logArr), 2, "success path should have 2 log entries (req + resp)")
	assert.Equal(t, logArr[0].Key, "svc-test-req")
	assert.Equal(t, logArr[1].Key, "svc-test-resp")
}

func TestCallApiJSONWithOpts_ConfigurableLogKeysFailure(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-fail",
			LogKeys: handlers.CallApiLogKeys{
				Request:  "svc-test-req",
				Response: "svc-test-resp",
				Failure:  "svc-test-fail",
			},
		},
	)

	assert.Assert(t, err != nil)
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok)
	assert.Equal(t, len(logArr), 2, "failure path should have 2 log entries (req + fail)")
	assert.Equal(t, logArr[0].Key, "svc-test-req")
	assert.Equal(t, logArr[1].Key, "svc-test-fail")
}

func TestCallApiJSONWithOpts_ConfigurableLogKeysEmptyFallback(t *testing.T) {
	_, param := setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-fallback",
			LogKeys: handlers.CallApiLogKeys{
				Request: "",
			},
		},
	)

	assert.NilError(t, err)
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok)
	assert.Equal(t, logArr[0].Key, "test-fallback", "empty Request key should fall back to Method")
	assert.Equal(t, logArr[1].Key, "test-fallback-resp", "empty Response key should fall back to default")
}

func TestCallApiJSONWithOpts_NormalizeErrorOptIn(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	recorder := &fakeMetricsRecorder{}
	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-normalize",
			MetricsRecorder: recorder,
			NormalizeError:  handlers.NormalizeCallError,
		},
	)

	assert.Assert(t, err != nil)
	var rce *libCallApi.RemoteCallError
	assert.Assert(t, !errors.As(err, &rce), "normalized error should not be RemoteCallError")

	assert.Equal(t, len(recorder.calls), 1)
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusForbidden)
	assert.Equal(t, recorder.calls[0].outcome, "failure")
	assert.Equal(t, logger.calls[0].StatusCode, http.StatusForbidden)

	var origRCE *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(logger.calls[0].Error, &origRCE),
		"transaction logger should retain original RemoteCallError")
}

func TestCallApiJSONWithOpts_NormalizeErrorDefault(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-no-normalize",
		},
	)

	assert.Assert(t, err != nil)
	var rce *libCallApi.RemoteCallError
	assert.Assert(t, errors.As(err, &rce), "default error should be RemoteCallError")
	assert.Equal(t, rce.Status, http.StatusForbidden)
}

func TestCallApiJSONWithOpts_RetrySuccessOnSecondAttempt(t *testing.T) {
	setupOptsTest(t) // for env setup
	w := libContext.InitContextNoAuditTrail(t)

	var serverAttempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&serverAttempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":[],"count":0}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method: "GET",
		Path:   "test1",
	}

	recorder := &fakeMetricsRecorder{}
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:          "test-retry",
			MetricsRecorder: recorder,
			RetryPolicy: &libRetry.RetryPolicy{
				MaxRetries:    1,
				RetryOnStatus: map[int]bool{http.StatusServiceUnavailable: true},
				Backoff:       0,
			},
		},
	)

	assert.NilError(t, err, "should succeed on second attempt")
	assert.Equal(t, atomic.LoadInt32(&serverAttempts), int32(2), "server should be called twice")
	assert.Equal(t, len(recorder.calls), 2, "should record metrics for both attempts")
	assert.Equal(t, recorder.calls[0].outcome, "failure")
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusServiceUnavailable)
	assert.Equal(t, recorder.calls[1].outcome, "success")
	assert.Equal(t, recorder.calls[1].statusCode, http.StatusOK)
}

func TestCallApiJSONWithOpts_RetryLogKeysPerAttempt(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	var serverAttempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&serverAttempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":[],"count":0}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method: "GET",
		Path:   "test1",
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "svc-call",
			RetryPolicy: &libRetry.RetryPolicy{
				MaxRetries:    1,
				RetryOnStatus: map[int]bool{http.StatusServiceUnavailable: true},
				Backoff:       0,
			},
		},
	)

	assert.NilError(t, err)
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok)
	// Attempt 1: req + fail = 2 entries
	// Attempt 2: req-retry-1 + resp-retry-1 = 2 entries
	assert.Equal(t, len(logArr), 4, "should have 4 log entries across 2 attempts")
	assert.Equal(t, logArr[0].Key, "svc-call")
	assert.Equal(t, logArr[1].Key, "svc-call-error")
	assert.Equal(t, logArr[2].Key, "svc-call-retry-1")
	assert.Equal(t, logArr[3].Key, "svc-call-retry-1-resp")
}

func TestCallApiJSONWithOpts_RetryExhausted(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	var serverAttempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverAttempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":"unavailable"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method: "GET",
		Path:   "test1",
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "svc-call",
			RetryPolicy: &libRetry.RetryPolicy{
				MaxRetries:    2,
				RetryOnStatus: map[int]bool{http.StatusServiceUnavailable: true},
				Backoff:       0,
			},
		},
	)

	assert.Assert(t, err != nil)
	assert.Equal(t, atomic.LoadInt32(&serverAttempts), int32(3), "server should be called 3 times")
}

func TestCallApiJSONWithOpts_RetryQueryStackPinned(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	var capturedQueries []string
	var serverAttempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverAttempts, 1)
		capturedQueries = append(capturedQueries, r.URL.RawQuery)
		if atomic.LoadInt32(&serverAttempts) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":[],"count":0}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:        libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:     "GET",
		Path:       "test1",
		QueryStack: &[]string{"?page=2&limit=5"},
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "svc-call",
			RetryPolicy: &libRetry.RetryPolicy{
				MaxRetries:    1,
				RetryOnStatus: map[int]bool{http.StatusServiceUnavailable: true},
				Backoff:       0,
			},
		},
	)

	assert.NilError(t, err)
	assert.Equal(t, len(capturedQueries), 2)
	assert.Equal(t, capturedQueries[0], "page=2&limit=5", "first attempt should use pinned query")
	assert.Equal(t, capturedQueries[1], "page=2&limit=5", "second attempt should use same pinned query")
}

func TestCallApiJSONWithOpts_TimeoutErrorErrorsIs(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:  "test-timeout",
			Timeout: 10 * time.Millisecond,
		},
	)

	assert.Assert(t, err != nil)
	assert.Assert(t, errors.Is(err, handlers.ErrServerTimeout),
		"timeout error should match ErrServerTimeout sentinel")
}

func TestCallApiJSONWithOpts_RetryWithFailedSuffix(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	var serverAttempts int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&serverAttempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success","data":[],"count":0}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method: "GET",
		Path:   "test1",
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "svc-call",
			LogKeys: handlers.CallApiLogKeys{
				Request:  "svc-call-req",
				Response: "svc-call-req-resp",
				Failure:  "svc-call-req-failed",
			},
			RetryPolicy: &libRetry.RetryPolicy{
				MaxRetries:    1,
				RetryOnStatus: map[int]bool{http.StatusServiceUnavailable: true},
				Backoff:       0,
			},
		},
	)

	assert.NilError(t, err)
	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok)
	// Attempt 1: req + failed = 2 entries
	// Attempt 2: req-retry-1 + resp-retry-1 = 2 entries
	assert.Equal(t, len(logArr), 4, "should have 4 log entries across 2 attempts")
	assert.Equal(t, logArr[0].Key, "svc-call-req")
	assert.Equal(t, logArr[1].Key, "svc-call-req-failed")
	assert.Equal(t, logArr[2].Key, "svc-call-req-retry-1")
	assert.Equal(t, logArr[3].Key, "svc-call-req-retry-1-resp")
}

func TestCallApiJSONWithOpts_TimeoutWithCustomStatus(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:            "test-timeout-500",
			Timeout:           10 * time.Millisecond,
			TimeoutStatusCode: http.StatusInternalServerError,
		},
	)

	assert.Assert(t, err != nil)
	ok, libErr := libError.Unwrap(err)
	assert.Assert(t, ok, "timeout error should be a libError")
	assert.Equal(t, int(libErr.Action().Status), http.StatusInternalServerError,
		"returned timeout libError should carry the configured 500 status")
	assert.Assert(t, errors.Is(err, handlers.ErrServerTimeout), "should still match sentinel")
}

func TestCallApiJSONWithOpts_TimeoutDefaultStatus(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:            "test-timeout-default",
			Timeout:           10 * time.Millisecond,
			TimeoutStatusCode: 0,
		},
	)

	assert.Assert(t, err != nil)
	ok, libErr := libError.Unwrap(err)
	assert.Assert(t, ok)
	assert.Equal(t, int(libErr.Action().Status), http.StatusRequestTimeout,
		"zero TimeoutStatusCode should default to 408")
}

func TestCallApiJSONWithOpts_TimeoutStatusDoesNotOverrideObserved(t *testing.T) {
	fakeServer, param := setupOptsTest(t)
	param.Path = "slow"
	param.Api.Domain = fakeServer.URL() + "/api"
	w := libContext.InitContextNoAuditTrail(t)

	recorder := &fakeMetricsRecorder{}
	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	var onCompleteInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:            "test-timeout-observed",
			Timeout:           10 * time.Millisecond,
			TimeoutStatusCode: http.StatusInternalServerError,
			MetricsRecorder:   recorder,
			OnComplete: func(info handlers.ApiCallInfo) {
				onCompleteInfo = info
			},
		},
	)

	assert.Assert(t, err != nil)
	// The returned libError status should be the configured 500.
	ok, libErr := libError.Unwrap(err)
	assert.Assert(t, ok)
	assert.Equal(t, int(libErr.Action().Status), http.StatusInternalServerError,
		"returned timeout libError should carry configured 500 status")

	// TransactionInfo.StatusCode should reflect the actual observed HTTP
	// status (200 from the slow endpoint), NOT the configured timeout status.
	assert.Equal(t, logger.calls[0].StatusCode, http.StatusOK,
		"TransactionInfo.StatusCode should be the observed 200, not the timeout 500")
	assert.Equal(t, onCompleteInfo.StatusCode, http.StatusOK,
		"OnComplete info StatusCode should be the observed 200, not the timeout 500")

	// Metrics should also retain the observed status.
	assert.Equal(t, recorder.calls[0].statusCode, http.StatusOK,
		"metrics statusCode should be the observed 200, not the timeout 500")
	assert.Equal(t, recorder.calls[0].outcome, "timeout")
}

// maskTestRequest is a request body type with a sensitive string field used
// to verify MaskFunc transforms TransactionInfo.Request without affecting
// AddLog attrs or the outbound request.
type maskTestRequest struct {
	PAN  string `json:"pan"`
	Name string `json:"name"`
}

// maskTestResponse is a response body type with a sensitive string field used
// to verify MaskFunc transforms TransactionInfo.Response without affecting
// the typed response returned by CallApiJSONWithOpts.
type maskTestResponse struct {
	Token   string `json:"token"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (s maskTestResponse) SetStatus(int)                {}
func (s maskTestResponse) SetHeaders(map[string]string) {}

// maskFunc wraps string values in "***" and returns other types unchanged.
// It is used to verify which fields MaskFunc is applied to.
func maskFunc(v any) any {
	switch s := v.(type) {
	case string:
		return "***" + s + "***"
	case []byte:
		return []byte("***" + string(s) + "***")
	case maskTestRequest:
		s.PAN = "***" + s.PAN + "***"
		return s
	case maskTestResponse:
		s.Token = "***" + s.Token + "***"
		return s
	default:
		return v
	}
}

func TestCallApiJSONWithOpts_MaskFuncApplied(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mask", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"secret-token","status":"ok","message":"hello"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param := &libCallApi.RemoteCallParamData[maskTestRequest, maskTestResponse]{
		Api:      libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:   "POST",
		Path:     "mask",
		JsonBody: reqBody,
	}

	var capturedInfo handlers.ApiCallInfo
	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:   "test-mask",
			MaskFunc: maskFunc,
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	// OnComplete info should have masked Request and Response.
	maskedReq, ok := capturedInfo.Request.(maskTestRequest)
	assert.Assert(t, ok, "Request should be maskTestRequest")
	assert.Equal(t, maskedReq.PAN, "***1234567890123456***", "Request.PAN should be masked")

	maskedResp, ok := capturedInfo.Response.(maskTestResponse)
	assert.Assert(t, ok, "Response should be maskTestResponse")
	assert.Equal(t, maskedResp.Token, "***secret-token***", "Response.Token should be masked")

	// The typed response returned by CallApiJSONWithOpts should be unmasked.
	assert.Equal(t, resp.Token, "secret-token", "returned response should be unmasked")
}

func TestCallApiJSONWithOpts_MaskFuncNil(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mask", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"secret-token","status":"ok","message":"hello"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param := &libCallApi.RemoteCallParamData[maskTestRequest, maskTestResponse]{
		Api:      libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:   "POST",
		Path:     "mask",
		JsonBody: reqBody,
	}

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method: "test-mask-nil",
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.NilError(t, err)
	rawReq, ok := capturedInfo.Request.(maskTestRequest)
	assert.Assert(t, ok)
	assert.Equal(t, rawReq.PAN, "1234567890123456", "nil MaskFunc should pass raw Request through")

	rawResp, ok := capturedInfo.Response.(maskTestResponse)
	assert.Assert(t, ok)
	assert.Equal(t, rawResp.Token, "secret-token", "nil MaskFunc should pass raw Response through")
	assert.Assert(t, capturedInfo.MaskedResponseBody == nil, "MaskedResponseBody should be nil when MaskFunc is nil")
}

func TestCallApiJSONWithOpts_MaskFuncAppliedOnError(t *testing.T) {
	_, param := setupOptsTest(t)
	param.Path = "forbidden"
	w := libContext.InitContextNoAuditTrail(t)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param.JsonBody = reqBody

	var capturedInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:   "test-mask-error",
			MaskFunc: maskFunc,
			OnComplete: func(info handlers.ApiCallInfo) {
				capturedInfo = info
			},
		},
	)

	assert.Assert(t, err != nil)
	// Request should be masked.
	maskedReq, ok := capturedInfo.Request.(maskTestRequest)
	assert.Assert(t, ok)
	assert.Equal(t, maskedReq.PAN, "***1234567890123456***", "Request.PAN should be masked on error path")

	// Response should be nil on error.
	assert.Assert(t, capturedInfo.Response == nil, "Response should be nil on error path")

	// ResponseBody should retain raw bytes; MaskedResponseBody should be masked.
	assert.Assert(t, len(capturedInfo.ResponseBody) > 0, "raw ResponseBody should be present")
	assert.Assert(t, strings.Contains(string(capturedInfo.ResponseBody), "forbidden"),
		"raw ResponseBody should contain 'forbidden'")

	maskedBody, ok := capturedInfo.MaskedResponseBody.([]byte)
	assert.Assert(t, ok, "MaskedResponseBody should be []byte")
	assert.Assert(t, strings.Contains(string(maskedBody), "***"),
		"MaskedResponseBody should contain mask markers")
	assert.Assert(t, strings.Contains(string(maskedBody), "forbidden"),
		"MaskedResponseBody should still contain original content")
}

func TestCallApiJSONWithOpts_MaskFuncNotAppliedToAddLog(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mask", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"secret-token","status":"ok","message":"hello"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param := &libCallApi.RemoteCallParamData[maskTestRequest, maskTestResponse]{
		Api:      libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:   "POST",
		Path:     "mask",
		JsonBody: reqBody,
		Headers:  map[string]string{"Authorization": "Bearer super-secret"},
	}

	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:   "test-mask-addlog",
			MaskFunc: maskFunc,
		},
	)
	assert.NilError(t, err)

	logs := w.Parser.GetLocal("LOG_ARRAY_ApiCall")
	assert.Assert(t, logs != nil)
	logArr, ok := logs.([]slog.Attr)
	assert.Assert(t, ok)
	// Success path: req + resp = 2 entries.
	assert.Equal(t, len(logArr), 2)

	// The request log entry (logArr[0]) should be a slog.GroupValue containing
	// a nested "request" attr with the ORIGINAL unmasked request body.
	reqAttr := logArr[0]
	assert.Equal(t, reqAttr.Key, "test-mask-addlog")

	// Resolve the group children of the request log attr. The param implements
	// slog.LogValuer, so we need to call LogValue() to resolve the group.
	anyVal := reqAttr.Value.Any()
	lv, ok := anyVal.(slog.LogValuer)
	assert.Assert(t, ok, "request log attr should be a slog.LogValuer, got %T", anyVal)
	resolvedVal := lv.LogValue()
	assert.Equal(t, resolvedVal.Kind(), slog.KindGroup, "resolved value should be a group")
	groupVal := resolvedVal.Group()
	var foundRequestAttr *slog.Attr
	for i := range groupVal {
		if groupVal[i].Key == "request" {
			foundRequestAttr = &groupVal[i]
			break
		}
	}
	assert.Assert(t, foundRequestAttr != nil, "request group should contain a 'request' attr")

	// The nested request value should resolve to the original unmasked request.
	loggedReq, ok := foundRequestAttr.Value.Any().(maskTestRequest)
	assert.Assert(t, ok, "nested request attr should be the original maskTestRequest")
	assert.Equal(t, loggedReq.PAN, "1234567890123456",
		"AddLog request attr should be unmasked (MaskFunc must not apply to AddLog)")

	// Also verify Authorization masking by LogValue() is independent of MaskFunc.
	var foundHeadersAttr *slog.Attr
	for i := range groupVal {
		if groupVal[i].Key == "headers" {
			foundHeadersAttr = &groupVal[i]
			break
		}
	}
	assert.Assert(t, foundHeadersAttr != nil, "request group should contain a 'headers' attr")
	headersVal, ok := foundHeadersAttr.Value.Any().(map[string]string)
	assert.Assert(t, ok, "headers attr should be map[string]string")
	assert.Equal(t, headersVal["Authorization"], "[masked]",
		"LogValue() should mask Authorization independently of MaskFunc")
}

func TestCallApiJSONWithOpts_MaskFuncReturnedResponseIsolated(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mask", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"secret-token","status":"ok","message":"hello"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param := &libCallApi.RemoteCallParamData[maskTestRequest, maskTestResponse]{
		Api:      libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:   "POST",
		Path:     "mask",
		JsonBody: reqBody,
	}

	resp, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:   "test-mask-isolation",
			MaskFunc: maskFunc,
		},
	)

	assert.NilError(t, err)
	// The typed response returned by CallApiJSONWithOpts must be the original
	// unmasked value, even though MaskFunc transformed TransactionInfo.Response.
	assert.Equal(t, resp.Token, "secret-token",
		"returned typed response should be unmasked (MaskFunc must not mutate it)")
	assert.Equal(t, resp.Status, "ok")
}

func TestCallApiJSONWithOpts_MaskFuncLoggerAndCallbackReceiveMasked(t *testing.T) {
	setupOptsTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/mask", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"secret-token","status":"ok","message":"hello"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	reqBody := maskTestRequest{PAN: "1234567890123456", Name: "alice"}
	param := &libCallApi.RemoteCallParamData[maskTestRequest, maskTestResponse]{
		Api:      libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:   "POST",
		Path:     "mask",
		JsonBody: reqBody,
	}

	logger := &fakeTransactionLogger{}
	w.Parser.SetLocal(webFramework.TransactionLoggerLocalKey, logger)

	var callbackInfo handlers.ApiCallInfo
	_, err := handlers.CallApiJSONWithOpts(
		w, nil, param,
		handlers.CallApiOptions{
			Method:   "test-mask-logger-callback",
			MaskFunc: maskFunc,
			OnComplete: func(info handlers.ApiCallInfo) {
				callbackInfo = info
			},
		},
	)
	assert.NilError(t, err)

	// Both TransactionLogger and OnComplete should receive masked Request/Response.
	assert.Equal(t, len(logger.calls), 1, "TransactionLogger should be called once")
	loggerReq, ok := logger.calls[0].Request.(maskTestRequest)
	assert.Assert(t, ok)
	assert.Equal(t, loggerReq.PAN, "***1234567890123456***",
		"TransactionLogger should receive masked Request")
	loggerResp, ok := logger.calls[0].Response.(maskTestResponse)
	assert.Assert(t, ok)
	assert.Equal(t, loggerResp.Token, "***secret-token***",
		"TransactionLogger should receive masked Response")

	cbReq, ok := callbackInfo.Request.(maskTestRequest)
	assert.Assert(t, ok)
	assert.Equal(t, cbReq.PAN, "***1234567890123456***",
		"OnComplete should receive masked Request")
	cbResp, ok := callbackInfo.Response.(maskTestResponse)
	assert.Assert(t, ok)
	assert.Equal(t, cbResp.Token, "***secret-token***",
		"OnComplete should receive masked Response")
}
