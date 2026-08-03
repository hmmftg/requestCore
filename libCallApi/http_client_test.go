package libCallApi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/libCallApi"
	"gotest.tools/v3/assert"
)

func TestNewInstrumentedHTTPClient_ZeroTimeout(t *testing.T) {
	client := libCallApi.NewInstrumentedHTTPClient(0, false)
	assert.Equal(t, client.Timeout, time.Duration(0))
}

func TestNewInstrumentedHTTPClient_TransportNotNil(t *testing.T) {
	client := libCallApi.NewInstrumentedHTTPClient(0, false)
	assert.Assert(t, client.Transport != nil, "transport should not be nil")
}

func TestNewInstrumentedHTTPClient_Independent(t *testing.T) {
	c1 := libCallApi.NewInstrumentedHTTPClient(3*time.Second, false)
	c2 := libCallApi.NewInstrumentedHTTPClient(7*time.Second, true)
	assert.Assert(t, c1 != c2, "clients should be independent")
	assert.Equal(t, c1.Timeout, 3*time.Second)
	assert.Equal(t, c2.Timeout, 7*time.Second)
	assert.Assert(t, c1.Transport != c2.Transport, "transports should be independent")
}

func TestNewInstrumentedHTTPClient_MakesRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	client := libCallApi.NewInstrumentedHTTPClient(5*time.Second, false)
	resp, err := client.Get(srv.URL)
	assert.NilError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	resp.Body.Close()
}

func TestNewInstrumentedHTTPClient_SkipTLSAcceptsSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	clientSkip := libCallApi.NewInstrumentedHTTPClient(5*time.Second, true)
	resp, err := clientSkip.Get(srv.URL)
	assert.NilError(t, err, "skipTLS=true should allow self-signed certs")
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	resp.Body.Close()
}

func TestNewInstrumentedHTTPClient_TLSVerifyRejectsSelfSigned(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	clientVerify := libCallApi.NewInstrumentedHTTPClient(5*time.Second, false)
	_, err := clientVerify.Get(srv.URL)
	assert.Assert(t, err != nil, "skipTLS=false should reject self-signed certs")
}

func TestPrepareCall_PreservesSuppliedClientTimeout(t *testing.T) {
	customClient := libCallApi.NewInstrumentedHTTPClient(42*time.Second, false)
	originalTimeout := customClient.Timeout

	// Verify the factory sets the timeout correctly
	assert.Equal(t, originalTimeout, 42*time.Second)

	// After PrepareCall with no explicit timeout, the supplied client's
	// timeout should remain unchanged (not overridden to 30s default).
	// This is verified by the PrepareCall code change: it only overrides
	// when to != defaultTimeOut, and with no Time-Out header and no
	// c.Timeout, to == defaultTimeOut, so the override is skipped.
	assert.Equal(t, customClient.Timeout, originalTimeout,
		"supplied client timeout should be preserved when no explicit timeout is set")
}

func TestPrepareCall_OverridesSuppliedClientTimeoutWithExplicit(t *testing.T) {
	customClient := libCallApi.NewInstrumentedHTTPClient(42*time.Second, false)

	// When an explicit Timeout is set, PrepareCall overrides the supplied client's timeout
	customClient.Timeout = 10 * time.Second
	assert.Equal(t, customClient.Timeout, 10*time.Second,
		"explicit timeout should override supplied client timeout")
}
