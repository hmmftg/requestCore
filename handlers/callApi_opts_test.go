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
		{"trailing slash domain", "http://example.com/", "api/test", "", "http://example.com/api/test"},
		{"leading slash path", "http://example.com", "/api/test", "", "http://example.com/api/test"},
		{"both slashes", "http://example.com/", "/api/test", "", "http://example.com/api/test"},
		{"no slashes", "http://example.com", "api/test", "", "http://example.com/api/test"},
		{"query with ?", "http://example.com", "api/test", "?page=1", "http://example.com/api/test?page=1"},
		{"query without ?", "http://example.com", "api/test", "page=1", "http://example.com/api/test?page=1"},
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
