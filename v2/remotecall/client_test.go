package remotecall_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/internal/restyadapter"
	"github.com/hmmftg/requestCore/v2/remotecall"
)

// testServer creates an httptest.Server with configurable behavior.
type testServer struct {
	*httptest.Server
	mu         sync.Mutex
	requests   []serverRequest
	statusCode int
	body       string
	handler    func(w http.ResponseWriter, r *http.Request)
}

type serverRequest struct {
	Method string
	Path   string
	Body   string
}

func newTestServer(statusCode int, body string) *testServer {
	ts := &testServer{statusCode: statusCode, body: body}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		bodyBytes, _ := io.ReadAll(r.Body)
		ts.requests = append(ts.requests, serverRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   string(bodyBytes),
		})
		ts.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.statusCode)
		fmt.Fprint(w, ts.body)
	}))
	return ts
}

func newCustomTestServer(handler func(w http.ResponseWriter, r *http.Request)) *testServer {
	ts := &testServer{handler: handler}
	ts.Server = httptest.NewServer(http.HandlerFunc(handler))
	return ts
}

func (ts *testServer) RequestCount() int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.requests)
}

func newTestClient(t *testing.T, httpClient *http.Client) *remotecall.RemoteClient {
	t.Helper()
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	// Register the adapter factory (normally done by init)
	remotecall.RegisterDefaultAdapterFactory(func(hc *http.Client) remotecall.Adapter {
		return restyadapter.New(hc)
	})
	return remotecall.NewRemoteClient(
		remotecall.WithHTTPClient(httpClient),
		remotecall.WithAppName("test-app"),
	)
}

type testResponse struct {
	Message string `json:"message"`
}

type testRequest struct {
	Name string `json:"name"`
}

func TestCall_Success(t *testing.T) {
	ts := newTestServer(200, `{"message":"hello"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	resp, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "hello" {
		t.Errorf("expected message 'hello', got %q", resp.Message)
	}
}

func TestCall_HTTPError(t *testing.T) {
	ts := newTestServer(500, `{"error":"internal"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T: %v", err, err)
	}
	if rce.Kind != remotecall.ErrorKindHTTP {
		t.Errorf("expected ErrorKindHTTP, got %v", rce.Kind)
	}
	if rce.StatusCode != 500 {
		t.Errorf("expected status 500, got %d", rce.StatusCode)
	}
	if string(rce.ResponseBody) != `{"error":"internal"}` {
		t.Errorf("expected response body, got %q", string(rce.ResponseBody))
	}
}

func TestCall_TransportError(t *testing.T) {
	// Use a client that will fail to connect
	client := newTestClient(t, &http.Client{
		Timeout: 1 * time.Second,
	})

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: "http://127.0.0.1:1"},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T: %v", err, err)
	}
	if rce.Kind != remotecall.ErrorKindTransport {
		t.Errorf("expected ErrorKindTransport, got %v", rce.Kind)
	}
}

func TestCall_DecodeError(t *testing.T) {
	ts := newTestServer(200, `{"invalid json`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T: %v", err, err)
	}
	if rce.Kind != remotecall.ErrorKindDecode {
		t.Errorf("expected ErrorKindDecode, got %v", rce.Kind)
	}
}

func TestCall_ContextCancellation(t *testing.T) {
	ts := newCustomTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	})
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := newTestClient(t, ts.Client())

	_, err := client.Call(ctx, remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be a preflight error since context was cancelled before execute
	if !errors.Is(err, remotecall.ErrPreflight) {
		// Or it could be a context error from the adapter
		var rce *remotecall.RemoteCallError
		if errors.As(err, &rce) {
			if rce.Kind != remotecall.ErrorKindContext {
				t.Errorf("expected ErrorKindContext, got %v", rce.Kind)
			}
		}
	}
}

func TestCall_JSONBody(t *testing.T) {
	var receivedBody string
	ts := newCustomTestServer(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"message":"ok"}`)
	})
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	resp, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "POST",
		Path:     "/test",
		BodyType: remotecall.BodyTypeJSON,
		Body:     testRequest{Name: "test"},
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "ok" {
		t.Errorf("expected message 'ok', got %q", resp.Message)
	}

	expected, _ := json.Marshal(testRequest{Name: "test"})
	if receivedBody != string(expected) {
		t.Errorf("expected body %q, got %q", string(expected), receivedBody)
	}
}

func TestCall_FormBody(t *testing.T) {
	ts := newCustomTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content-type, got %q", r.Header.Get("Content-Type"))
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %q", r.Form.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"message":"ok"}`)
	})
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	resp, err := client.Call(context.Background(), remotecall.CallConfig[url.Values, testResponse]{
		API: remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method: "POST",
		Path: "/test",
		BodyType: remotecall.BodyTypeForm,
		Body: url.Values{
			"grant_type": {"client_credentials"},
			"scope":      {"payments"},
		},
		Builder: remotecall.DefaultBuilder[testResponse]{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "ok" {
		t.Errorf("expected message 'ok', got %q", resp.Message)
	}
}

func TestCall_FormBody_InvalidType(t *testing.T) {
	ts := newTestServer(200, `{"message":"ok"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[map[string]string, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "POST",
		Path:     "/test",
		BodyType: remotecall.BodyTypeForm,
		Body:     map[string]string{"grant_type": "client_credentials"},
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error for non-url.Values form body, got nil")
	}
	if !errors.Is(err, remotecall.ErrPreflight) {
		t.Errorf("expected ErrPreflight, got %v", err)
	}

	// Verify no HTTP request was made
	if ts.RequestCount() != 0 {
		t.Errorf("expected 0 requests, got %d", ts.RequestCount())
	}
}

func TestCall_RawResponseBodyOwnership(t *testing.T) {
	ts := newTestServer(500, `{"error":"foo"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T", err)
	}

	original := string(rce.ResponseBody)

	// Mutate the response body
	rce.ResponseBody[0] = 'X'

	// Make another call — the first response should be unaffected
	_, err2 := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err2 == nil {
		t.Fatal("expected error on second call")
	}

	var rce2 *remotecall.RemoteCallError
	if !errors.As(err2, &rce2) {
		t.Fatalf("expected RemoteCallError on second call, got %T", err2)
	}

	// The first response body should not have been affected by our mutation
	// (we mutated our copy, not the adapter's buffer)
	if string(rce2.ResponseBody) != original {
		t.Errorf("response body ownership violated: expected %q, got %q", original, string(rce2.ResponseBody))
	}
}

func TestCall_Skipped(t *testing.T) {
	ts := newTestServer(200, `{"message":"ok"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API: remotecall.RemoteAPI{
			Name:         "test-api",
			BaseURL:      ts.URL,
			SkipPatterns: []string{"test"},
		},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if !errors.Is(err, remotecall.ErrSkipped) {
		t.Fatalf("expected ErrSkipped, got %v", err)
	}
	if ts.RequestCount() != 0 {
		t.Errorf("expected 0 requests for skipped call, got %d", ts.RequestCount())
	}
}

func TestCall_CustomHTTPClient(t *testing.T) {
	ts := newTestServer(200, `{"message":"ok"}`)
	defer ts.Close()

	customClient := ts.Client()
	client := newTestClient(t, customClient)

	resp, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message != "ok" {
		t.Errorf("expected message 'ok', got %q", resp.Message)
	}
}

func TestCall_4xxIsHTTPError(t *testing.T) {
	ts := newTestServer(404, `{"error":"not found"}`)
	defer ts.Close()

	client := newTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[testRequest, testResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[testResponse]{},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T", err)
	}
	if rce.Kind != remotecall.ErrorKindHTTP {
		t.Errorf("expected ErrorKindHTTP for 404, got %v", rce.Kind)
	}
	if rce.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", rce.StatusCode)
	}
}
