package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/hmmftg/requestCore/handlers"
	"github.com/hmmftg/requestCore/libCallApi"
	"github.com/hmmftg/requestCore/libContext"
	"gotest.tools/v3/assert"
)

func setupHeadersTest(t *testing.T) {
	t.Helper()
	t.Setenv(libContext.HeaderEnvKey, "User-Id#a@b#b")
	t.Setenv(libContext.LocalEnvKey, "User-Id#a@b#b")
}

func TestBuildBaseRemoteHeaders_Basic(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	headers := handlers.BuildBaseRemoteHeaders(w, "myapp", "")

	assert.Equal(t, headers["Accept"], "application/json")
	assert.Equal(t, headers["X-App-ID"], "myapp-"+strconv.Itoa(os.Getpid()))
	_, hasRID := headers[handlers.RequestIDHeader]
	assert.Assert(t, !hasRID, "should not have X-Request-ID when not set")
	_, hasCID := headers[handlers.CorrelationIDHeader]
	assert.Assert(t, !hasCID, "should not have X-Correlation-ID when not set")
}

func TestBuildBaseRemoteHeaders_WithRequestID(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.RequestIDLocalKey, "req-123-abc")

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	assert.Equal(t, headers[handlers.RequestIDHeader], "req-123-abc")
	assert.Equal(t, headers["Accept"], "application/json")
}

func TestBuildBaseRemoteHeaders_RequestIDWrongType(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.RequestIDLocalKey, 12345)

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	_, hasRID := headers[handlers.RequestIDHeader]
	assert.Assert(t, !hasRID, "should not have X-Request-ID when type is not string")
}

func TestBuildBaseRemoteHeaders_RequestIDEmpty(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.RequestIDLocalKey, "")

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	_, hasRID := headers[handlers.RequestIDHeader]
	assert.Assert(t, !hasRID, "should not have X-Request-ID when empty")
}

func TestBuildBaseRemoteHeaders_MapIsolation(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	h1 := handlers.BuildBaseRemoteHeaders(w, "app", "")
	h2 := handlers.BuildBaseRemoteHeaders(w, "app", "")

	h1["Custom"] = "value"
	_, ok := h2["Custom"]
	assert.Assert(t, !ok, "modifying one map should not affect the other")
}

func TestBuildBaseRemoteHeaders_WithCorrelationIDParam(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "corr-from-arg")

	assert.Equal(t, headers[handlers.CorrelationIDHeader], "corr-from-arg")
}

func TestBuildBaseRemoteHeaders_CorrelationIDParamEmptyFallsBackToLocal(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.CorrelationIDLocalKey, "corr-from-local")

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	assert.Equal(t, headers[handlers.CorrelationIDHeader], "corr-from-local")
}

func TestBuildBaseRemoteHeaders_ParamOverridesLocal(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.CorrelationIDLocalKey, "corr-from-local")

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "corr-from-arg")

	assert.Equal(t, headers[handlers.CorrelationIDHeader], "corr-from-arg",
		"non-empty arg should take precedence over parser local")
}

func TestBuildBaseRemoteHeaders_BothRequestAndCorrelationID(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.RequestIDLocalKey, "req-123")

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "corr-456")

	assert.Equal(t, headers[handlers.RequestIDHeader], "req-123")
	assert.Equal(t, headers[handlers.CorrelationIDHeader], "corr-456")
}

func TestBuildBaseRemoteHeaders_CorrelationIDLocalWrongType(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)
	w.Parser.SetLocal(handlers.CorrelationIDLocalKey, 12345)

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	_, hasCID := headers[handlers.CorrelationIDHeader]
	assert.Assert(t, !hasCID, "should not have X-Correlation-ID when local type is not string")
}

func TestBuildBaseRemoteHeaders_CorrelationIDEmpty(t *testing.T) {
	setupHeadersTest(t)
	w := libContext.InitContextNoAuditTrail(t)

	headers := handlers.BuildBaseRemoteHeaders(w, "svc", "")

	_, hasCID := headers[handlers.CorrelationIDHeader]
	assert.Assert(t, !hasCID, "should not have X-Correlation-ID when arg empty and no local set")
}

func TestPrepareCall_NoDuplicateAcceptHeader(t *testing.T) {
	setupHeadersTest(t)
	fakeServer := libCallApi.NewFakeAPIServer()
	t.Cleanup(fakeServer.Close)

	var observedAccept []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		observedAccept = r.Header["Accept"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	w := libContext.InitContextNoAuditTrail(t)
	param := &libCallApi.RemoteCallParamData[any, map[string]any]{
		Api:     libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method:  "GET",
		Path:    "test",
		Headers: map[string]string{"Accept": "application/vnd.custom+json"},
	}
	param.BodyType = libCallApi.JSON
	_, _ = libCallApi.RemoteCall(w, param)

	assert.Equal(t, len(observedAccept), 1, "should have exactly one Accept header")
	assert.Equal(t, observedAccept[0], "application/vnd.custom+json")
}

func TestPrepareCall_DefaultAcceptWhenNotProvided(t *testing.T) {
	setupHeadersTest(t)
	fakeServer := libCallApi.NewFakeAPIServer()
	t.Cleanup(fakeServer.Close)

	var observedAccept []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/test1", func(w http.ResponseWriter, r *http.Request) {
		observedAccept = r.Header["Accept"]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success","data":[],"count":0}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	w := libContext.InitContextNoAuditTrail(t)
	param := &libCallApi.RemoteCallParamData[any, optsTestResponse]{
		Api:    libCallApi.RemoteApi{Domain: srv.URL + "/api"},
		Method: "GET",
		Path:   "test1",
	}
	param.BodyType = libCallApi.JSON
	_, _ = libCallApi.RemoteCall(w, param)

	assert.Equal(t, len(observedAccept), 1, "should have exactly one Accept header")
	assert.Equal(t, observedAccept[0], "application/json")
}
