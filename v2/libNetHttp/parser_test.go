package libNetHttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

func TestNetHTTPParserV2_SendResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	parser := InitContextV2(r, w)
	err := parser.SendResponse(200, "application/json", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("SendResponse: %v", err)
	}

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %s", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != `{"ok":true}` {
		t.Fatalf("expected {\"ok\":true}, got %s", w.Body.String())
	}
}

func TestNetHTTPParserV2_GetCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	parser := InitContextV2(r, httptest.NewRecorder())
	val := parser.GetCookie("session")
	if val != "abc123" {
		t.Fatalf("expected 'abc123', got %q", val)
	}

	missing := parser.GetCookie("nonexistent")
	if missing != "" {
		t.Fatalf("expected empty string, got %q", missing)
	}
}

func TestNetHTTPParserV2_SetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)

	parser := InitContextV2(r, w)
	parser.SetCookie(&http.Cookie{Name: "test", Value: "xyz"})

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "test" || cookies[0].Value != "xyz" {
		t.Fatalf("expected test=xyz, got %s=%s", cookies[0].Name, cookies[0].Value)
	}
}

func TestNetHTTPRouter_BasicRoute(t *testing.T) {
	router := NewRouter()

	called := false
	err := router.Get("/users", func(ctx *request.Context, transport routing.Transport) error {
		called = true
		return transport.WriteResponse(200, "text/plain", nil, []byte("hello"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/users")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatal("expected handler to be called")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Fatalf("expected 'hello', got %s", string(body))
	}
}

func TestNetHTTPRouter_ParamRoute(t *testing.T) {
	router := NewRouter()

	err := router.Get("/users/{id}", func(ctx *request.Context, transport routing.Transport) error {
		id := ctx.PathParam("id")
		return transport.WriteResponse(200, "text/plain", nil, []byte(id))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/users/123")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "123" {
		t.Fatalf("expected '123', got %s", string(body))
	}
}

func TestNetHTTPRouter_Group(t *testing.T) {
	router := NewRouter()

	api := router.Group("/api")
	called := false
	err := api.Get("/users", func(ctx *request.Context, transport routing.Transport) error {
		called = true
		return transport.WriteResponse(200, "text/plain", nil, []byte("ok"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/users")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatal("expected handler to be called")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNetHTTPRouter_Middleware(t *testing.T) {
	router := NewRouter()

	order := []string{}
	mw := func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			order = append(order, "mw-before")
			err := next(ctx, transport)
			order = append(order, "mw-after")
			return err
		}
	}

	router.With(mw).Get("/test", func(ctx *request.Context, transport routing.Transport) error {
		order = append(order, "handler")
		return transport.WriteResponse(200, "text/plain", nil, []byte("ok"))
	})

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	expected := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d entries, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
		}
	}
}

func TestNetHTTPRouter_InvalidPattern(t *testing.T) {
	router := NewRouter()

	err := router.Get("/users/{id", func(ctx *request.Context, transport routing.Transport) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestNetHTTPRouter_Native(t *testing.T) {
	router := NewRouter()
	if router.Native() == nil {
		t.Fatal("expected non-nil Native()")
	}
}

func TestNetHTTPRouter_HandlerError(t *testing.T) {
	router := NewRouter()

	router.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = transport.WriteResponse(http.StatusInternalServerError, "text/plain", nil, []byte(err.Error()))
	})

	router.Get("/fail", func(ctx *request.Context, transport routing.Transport) error {
		ctx2, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx2.Err()
	})

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/fail")
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetHTTPRequest(t *testing.T) {
	router := NewRouter()

	var extracted *http.Request
	router.Get("/test", func(ctx *request.Context, transport routing.Transport) error {
		extracted, _ = ctx.Native().(*http.Request)
		return transport.WriteResponse(200, "text/plain", nil, []byte("ok"))
	})

	server := httptest.NewServer(router.mux)
	defer server.Close()

	http.Get(server.URL + "/test")
	if extracted == nil {
		t.Fatal("expected non-nil *http.Request from GetHTTPRequest")
	}
}

func TestNetHTTPRouter_MethodNotAllowed(t *testing.T) {
	router := NewRouter()

	router.MethodNotAllowed(func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusMethodNotAllowed, "text/plain", nil, []byte("method not allowed"))
	})

	router.Post("/items", func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusOK, "text/plain", nil, []byte("created"))
	})

	// Use Native() which wraps with 405 interception.
	handler, ok := router.Native().(http.Handler)
	if !ok {
		t.Fatal("expected Native() to implement http.Handler")
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/items")
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "method not allowed" {
		t.Fatalf("expected 'method not allowed', got %s", string(body))
	}
}
