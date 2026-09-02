package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hmmftg/requestCore/response"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/workers"
)

func TestBootstrap_Chi(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Router == nil {
		t.Fatal("expected non-nil router")
	}
	if app.RespHandler == nil {
		t.Fatal("expected non-nil response handler")
	}
	if app.Worker == nil {
		t.Fatal("expected non-nil worker")
	}
	if app.Sessions == nil {
		t.Fatal("expected non-nil session manager")
	}
}

func TestBootstrap_Gin(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkGin,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Router == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestBootstrap_Fiber(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkFiber,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Router == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestBootstrap_NetHTTP(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkNetHTTP,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Router == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestBootstrap_UnsupportedFramework(t *testing.T) {
	_, err := Bootstrap(Config{
		Framework: Framework("unknown"),
	})
	if err == nil {
		t.Fatal("expected error for unsupported framework")
	}
}

func TestBootstrap_DefaultRenderer(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Renderer.ContentType() != "application/json" {
		t.Fatalf("expected default JSON renderer, got %s", app.Renderer.ContentType())
	}
}

func TestBootstrap_CustomRenderer(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
		Renderer:  renderers.XMLRenderer{},
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.Renderer.ContentType() != "application/xml" {
		t.Fatalf("expected XML renderer, got %s", app.Renderer.ContentType())
	}
}

func TestBootstrap_WithLegacyHandler(t *testing.T) {
	legacy := response.WebHanlder{
		MessageDesc: map[string]string{"test": "Test message"},
		ErrorDesc:   map[string]string{"test": "Test error"},
	}
	app, err := Bootstrap(Config{
		Framework:     FrameworkChi,
		LegacyHandler: legacy,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	if app.RespHandler.LegacyHandler().MessageDesc["test"] != "Test message" {
		t.Fatal("expected legacy handler to be passed through")
	}
}

func TestApp_Register(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	group := app.Register("/api")
	if group == nil {
		t.Fatal("expected non-nil route group")
	}
}

func TestApp_RegisterRoute(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	// Register a simple route
	err = app.Router.Get("/test", func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{"status":"ok"}`))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Test the route
	native := app.Router.Native()
	mux, ok := native.(http.Handler)
	if !ok {
		t.Fatal("expected chi mux to implement http.Handler")
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status=ok, got %s", result["status"])
	}
}

func TestApp_StartShutdown(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	app.Router.Get("/health", func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{"status":"healthy"}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = app.StartWithContext(ctx, ":0")
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = app.Shutdown(shutdownCtx)
	cancel()
}

// TestApp_DefaultNotFound verifies that the default 404 handler emits
// a JSON error response when no custom NotFound is configured.
func TestApp_DefaultNotFound(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	native := app.Router.Native()
	mux, ok := native.(http.Handler)
	if !ok {
		t.Fatal("expected chi mux to implement http.Handler")
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["error"] != "not_found" {
		t.Fatalf("expected error='not_found', got %v", result["error"])
	}
}

// TestApp_DefaultMethodNotAllowed verifies that the default 405 handler
// emits a JSON error response when no custom MethodNotAllowed is configured.
func TestApp_DefaultMethodNotAllowed(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	defer app.Close()

	err = app.Router.Post("/items", func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{"status":"created"}`))
	})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}

	native := app.Router.Native()
	mux, ok := native.(http.Handler)
	if !ok {
		t.Fatal("expected chi mux to implement http.Handler")
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/items")
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["error"] != "method_not_allowed" {
		t.Fatalf("expected error='method_not_allowed', got %v", result["error"])
	}
}

// TestApp_ShutdownStopsWorker verifies that Shutdown also shuts down
// the worker pool, not just the HTTP server.
func TestApp_ShutdownStopsWorker(t *testing.T) {
	app, err := Bootstrap(Config{
		Framework: FrameworkChi,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	app.Router.Get("/health", func(ctx *request.Context, transport routing.Transport) error {
		return transport.WriteResponse(http.StatusOK, "application/json", nil, []byte(`{"status":"healthy"}`))
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = app.StartWithContext(ctx, ":0")
	}()
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	cancel()

	// After Shutdown, submitting a job should fail with ErrShutdown.
	err = app.Worker.Submit(context.Background(), workers.Job{
		Name: "post-shutdown",
		Handler: func(ctx *workers.JobContext) error {
			return nil
		},
	})
	if !errors.Is(err, workers.ErrShutdown) {
		t.Fatalf("expected ErrShutdown from worker after Shutdown, got %v", err)
	}
}
