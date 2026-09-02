package libChi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
)

func TestChiRouter_BasicRoute(t *testing.T) {
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

func TestChiRouter_ParamRoute(t *testing.T) {
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

func TestChiRouter_Group(t *testing.T) {
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

func TestChiRouter_Middleware(t *testing.T) {
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

func TestChiRouter_InvalidPattern(t *testing.T) {
	router := NewRouter()

	err := router.Get("/users/{id", func(ctx *request.Context, transport routing.Transport) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestChiRouter_Native(t *testing.T) {
	router := NewRouter()
	if router.Native() == nil {
		t.Fatal("expected non-nil Native()")
	}
}

func TestChiRouter_NotFound(t *testing.T) {
	router := NewRouter()

	router.NotFound(func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(404, "text/plain", nil, []byte("not found"))
	})

	server := httptest.NewServer(router.mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "not found" {
		t.Fatalf("expected 'not found', got %s", string(body))
	}
}

func TestChiRouter_HandlerError(t *testing.T) {
	router := NewRouter()
	router.SetErrorHandler(func(ctx *request.Context, transport routing.Transport, err error) {
		_ = transport.WriteResponse(500, "text/plain", nil, []byte(err.Error()))
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
