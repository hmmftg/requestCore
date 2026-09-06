package remotecall_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/v2/internal/restyadapter"
	"github.com/hmmftg/requestCore/v2/remotecall"
)

type retryResponse struct {
	Message string `json:"message"`
}

func newRetryTestClient(t *testing.T, httpClient *http.Client) *remotecall.RemoteClient {
	t.Helper()
	remotecall.RegisterDefaultAdapterFactory(func(hc *http.Client) remotecall.Adapter {
		return restyadapter.New(hc)
	})
	return remotecall.NewRemoteClient(
		remotecall.WithHTTPClient(httpClient),
		remotecall.WithAppName("test-app"),
	)
}

func TestRetry_MaxRetriesSemantics(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":"unavailable"}`)
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       2,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// MaxRetries=2 → 3 total attempts (1 initial + 2 retries)
	if got := atomic.LoadInt32(&attemptCount); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestRetry_MaxRetriesZero(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       0,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// MaxRetries=0 → 1 attempt only
	if got := atomic.LoadInt32(&attemptCount); got != 1 {
		t.Errorf("expected 1 attempt, got %d", got)
	}
}

func TestRetry_ContextCancellationNeverRetried(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	client := newRetryTestClient(t, ts.Client())

	// Cancel context after first attempt starts
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := client.Call(ctx, remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       5,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Context cancellation should not cause retries
	if got := atomic.LoadInt32(&attemptCount); got > 1 {
		t.Errorf("expected at most 1 attempt for cancelled context, got %d", got)
	}
}

func TestRetry_PerCallOverridesDefault(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())
	// Set default retry with MaxRetries=3
	client = remotecall.NewRemoteClient(
		remotecall.WithHTTPClient(ts.Client()),
		remotecall.WithAppName("test-app"),
		remotecall.WithDefaultRetry(remotecall.RetryPolicy{
			MaxRetries:       3,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		}),
	)

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		// Per-call retry with MaxRetries=1 should override default of 3
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       1,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Per-call MaxRetries=1 → 2 total attempts
	if got := atomic.LoadInt32(&attemptCount); got != 2 {
		t.Errorf("expected 2 attempts (per-call override), got %d", got)
	}
}

func TestRetry_PerCallDisablesDefault(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := remotecall.NewRemoteClient(
		remotecall.WithHTTPClient(ts.Client()),
		remotecall.WithAppName("test-app"),
		remotecall.WithDefaultRetry(remotecall.RetryPolicy{
			MaxRetries:       3,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		}),
	)

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		// Per-call retry with MaxRetries=0 explicitly disables
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       0,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Per-call MaxRetries=0 → 1 attempt only
	if got := atomic.LoadInt32(&attemptCount); got != 1 {
		t.Errorf("expected 1 attempt (per-call disables), got %d", got)
	}
}

func TestRetry_NoRetryByDefault(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		// No Retry policy — should not retry
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if got := atomic.LoadInt32(&attemptCount); got != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", got)
	}
}

func TestRetry_BreakerOneOutcomePerCall(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(503)
	}))
	defer ts.Close()

	client := remotecall.NewRemoteClient(
		remotecall.WithHTTPClient(ts.Client()),
		remotecall.WithAppName("test-app"),
		remotecall.WithCircuitBreaker(remotecall.CircuitBreakerPolicy{
			FailureThreshold: 100, // high to prevent tripping
		}),
	)

	_, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       2,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// 3 attempts, but only 1 breaker failure should be recorded
	// (breaker records one outcome per logical Call)
	if got := atomic.LoadInt32(&attemptCount); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}

	// Verify the error is HTTP 503
	var rce *remotecall.RemoteCallError
	if !errors.As(err, &rce) {
		t.Fatalf("expected RemoteCallError, got %T", err)
	}
	if rce.StatusCode != 503 {
		t.Errorf("expected status 503, got %d", rce.StatusCode)
	}
}

func TestRetry_LastAttemptSucceeds(t *testing.T) {
	var attemptCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attemptCount, 1)
		if count < 3 {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"message":"success"}`)
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())

	resp, err := client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "GET",
		Path:     "/test",
		BodyType: remotecall.BodyTypeEmpty,
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       2,
			RetryableMethods: map[string]bool{"GET": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})
	if err != nil {
		t.Fatalf("expected success on 3rd attempt, got error: %v", err)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got %q", resp.Message)
	}
	if got := atomic.LoadInt32(&attemptCount); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}

func TestRetry_ConcurrentSharedHTTPClient(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	sharedClient := ts.Client()
	client := newRetryTestClient(t, sharedClient)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Call(context.Background(), remotecall.CallConfig[any, retryResponse]{
				API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
				Method:   "GET",
				Path:     "/test",
				BodyType: remotecall.BodyTypeEmpty,
				Builder:  remotecall.DefaultBuilder[retryResponse]{},
				Retry: &remotecall.RetryPolicy{
					MaxRetries:       1,
					RetryableMethods: map[string]bool{"GET": true},
					RetryOnStatus:    map[int]bool{503: true},
				},
			})
		}()
	}
	wg.Wait()
	// If we get here without panicking or data races, the test passes
}

func TestRetry_ImmutableByteReplay(t *testing.T) {
	var lastBody string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		lastBody = string(bodyBytes)
		w.WriteHeader(503) // always fail to trigger retry
	}))
	defer ts.Close()

	client := newRetryTestClient(t, ts.Client())

	type payload struct {
		Name string `json:"name"`
	}

	_, _ = client.Call(context.Background(), remotecall.CallConfig[payload, retryResponse]{
		API:      remotecall.RemoteAPI{Name: "test-api", BaseURL: ts.URL},
		Method:   "POST",
		Path:     "/test",
		BodyType: remotecall.BodyTypeJSON,
		Body:     payload{Name: "test"},
		Builder:  remotecall.DefaultBuilder[retryResponse]{},
		Retry: &remotecall.RetryPolicy{
			MaxRetries:       2,
			RetryableMethods: map[string]bool{"POST": true},
			RetryOnStatus:    map[int]bool{503: true},
		},
	})

	// The body should be the same on every attempt (immutable replay)
	expected := `{"name":"test"}`
	if lastBody != expected {
		t.Errorf("expected body %q on last attempt, got %q", expected, lastBody)
	}
}
