// This file contains cross-framework conformance tests that verify all
// adapters behave consistently for core routing operations.
package routing_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	chi "github.com/go-chi/chi/v5"
	"github.com/gofiber/fiber/v2"

	v2libChi "github.com/hmmftg/requestCore/v2/libChi"
	v2libFiber "github.com/hmmftg/requestCore/v2/libFiber"
	v2libGin "github.com/hmmftg/requestCore/v2/libGin"
	v2libNetHttp "github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// AdapterFactory creates a router and provides a way to make HTTP requests
// against it for testing.
type AdapterFactory struct {
	Name      string
	NewRouter func() (router routing.Router, serveHTTP func(req *http.Request) (*http.Response, error))
}

func init() {
	gin.SetMode(gin.TestMode)
}

func adapterFactories() []AdapterFactory {
	return []AdapterFactory{
		{
			Name: "gin",
			NewRouter: func() (routing.Router, func(req *http.Request) (*http.Response, error)) {
				engine := gin.New()
				router := v2libGin.NewRouter(engine)
				return router, func(req *http.Request) (*http.Response, error) {
					w := httptest.NewRecorder()
					engine.ServeHTTP(w, req)
					return w.Result(), nil
				}
			},
		},
		{
			Name: "fiber",
			NewRouter: func() (routing.Router, func(req *http.Request) (*http.Response, error)) {
				app := fiber.New()
				router := v2libFiber.NewRouter(app)
				return router, func(req *http.Request) (*http.Response, error) {
					resp, err := app.Test(req)
					return resp, err
				}
			},
		},
		{
			Name: "chi",
			NewRouter: func() (routing.Router, func(req *http.Request) (*http.Response, error)) {
				router := v2libChi.NewRouter()
				mux := router.Native().(*chi.Mux)
				return router, func(req *http.Request) (*http.Response, error) {
					server := httptest.NewServer(mux)
					defer server.Close()
					// Create a new request to avoid RequestURI issues with httptest.NewRequest
					newReq, err := http.NewRequest(req.Method, server.URL+req.URL.Path, req.Body)
					if err != nil {
						return nil, err
					}
					newReq.Header = req.Header
					return http.DefaultClient.Do(newReq)
				}
			},
		},
		{
			Name: "nethttp",
			NewRouter: func() (routing.Router, func(req *http.Request) (*http.Response, error)) {
				router := v2libNetHttp.NewRouter()
				return router, func(req *http.Request) (*http.Response, error) {
					// Native() returns the intercept405-wrapped mux when a
					// MethodNotAllowed handler is configured, otherwise the
					// raw mux. Use a test server so the wrapped handler runs.
					server := httptest.NewServer(router.Native().(http.Handler))
					defer server.Close()
					newReq, err := http.NewRequest(req.Method, server.URL+req.URL.Path, req.Body)
					if err != nil {
						return nil, err
					}
					newReq.Header = req.Header
					return http.DefaultClient.Do(newReq)
				}
			},
		},
	}
}

func TestConformance_BasicRoute(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			called := false
			if err := router.Get("/test", func(ctx *v2wf.RequestContext) error {
				called = true
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			}); err != nil {
				t.Fatalf("Get: %v", err)
			}

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if !called {
				t.Fatal("expected handler to be called")
			}
			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "ok" {
				t.Fatalf("expected 'ok', got %s", string(body))
			}
		})
	}
}

func TestConformance_ParamRoute(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			if err := router.Get("/users/{id}", func(ctx *v2wf.RequestContext) error {
				id := ctx.Parser.GetURLParam("id")
				return ctx.Parser.SendResponse(200, "text/plain", []byte(id))
			}); err != nil {
				t.Fatalf("Get: %v", err)
			}

			req := httptest.NewRequest("GET", "/users/123", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "123" {
				t.Fatalf("expected '123', got %s", string(body))
			}
		})
	}
}

func TestConformance_Group(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			api := router.Group("/api")
			if err := api.Get("/users", func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(200, "text/plain", []byte("group-ok"))
			}); err != nil {
				t.Fatalf("Get: %v", err)
			}

			req := httptest.NewRequest("GET", "/api/users", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "group-ok" {
				t.Fatalf("expected 'group-ok', got %s", string(body))
			}
		})
	}
}

func TestConformance_MiddlewareOrder(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			order := []string{}
			mw := func(next routing.Handler) routing.Handler {
				return func(ctx *v2wf.RequestContext) error {
					order = append(order, "mw-before")
					err := next(ctx)
					order = append(order, "mw-after")
					return err
				}
			}

			router.With(mw).Get("/test", func(ctx *v2wf.RequestContext) error {
				order = append(order, "handler")
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			expected := []string{"mw-before", "handler", "mw-after"}
			if len(order) != len(expected) {
				t.Fatalf("expected %d entries, got %d: %v", len(expected), len(order), order)
			}
			for i, v := range expected {
				if order[i] != v {
					t.Fatalf("expected order[%d]=%q, got %q", i, v, order[i])
				}
			}
		})
	}
}

func TestConformance_HandlerError(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			router.Get("/fail", func(ctx *v2wf.RequestContext) error {
				ctx2, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx2.Err()
			})

			req := httptest.NewRequest("GET", "/fail", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 500 {
				t.Fatalf("expected 500, got %d", resp.StatusCode)
			}
		})
	}
}

func TestConformance_InvalidPattern(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, _ := af.NewRouter()

			err := router.Get("/users/{id", func(ctx *v2wf.RequestContext) error {
				return nil
			})
			if err == nil {
				t.Fatal("expected error for invalid pattern")
			}
		})
	}
}

// TestConformance_NoDoubleWrite verifies that when a handler writes a
// response directly and then returns an error, the adapter does not
// write a second response (the error is ignored because the response
// is already committed).
func TestConformance_NoDoubleWrite(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			router.Get("/test", func(ctx *v2wf.RequestContext) error {
				// Write a 200 response directly.
				_ = ctx.Parser.SendResponse(200, "text/plain", []byte("first"))
				// Return an error after committing.
				return errors.New("should be ignored")
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200 (first write wins), got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if string(body) != "first" {
				t.Fatalf("expected 'first', got %s", string(body))
			}
		})
	}
}

// TestConformance_HookRunsOnce verifies that before-commit hooks run
// exactly once per request, even when the handler writes directly.
func TestConformance_HookRunsOnce(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			hookCalls := 0
			router.Get("/test", func(ctx *v2wf.RequestContext) error {
				ctx.AddBeforeCommitHook(func(c *v2wf.RequestContext) error {
					hookCalls++
					return nil
				})
				// Direct write triggers the hook runner in SendResponse.
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			if hookCalls != 1 {
				t.Fatalf("expected hook to run exactly once, got %d", hookCalls)
			}
		})
	}
}

// TestConformance_204NoContent verifies that a 204 response with no body
// is handled correctly across all adapters.
func TestConformance_204NoContent(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			router.Get("/test", func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(204, "", nil)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 204 {
				t.Fatalf("expected 204, got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if len(body) != 0 {
				t.Fatalf("expected empty body for 204, got %d bytes", len(body))
			}
		})
	}
}

// TestConformance_404NotFound verifies that unmatched routes return 404
// when a NotFound handler is configured.
func TestConformance_404NotFound(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			router.Get("/exists", func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			})

			// Register a 404 handler.
			router.NotFound(func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(404, "text/plain", []byte("not-found"))
			})

			req := httptest.NewRequest("GET", "/nonexistent", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 404 {
				t.Fatalf("expected 404, got %d", resp.StatusCode)
			}
		})
	}
}

// TestConformance_405MethodNotAllowed verifies that using a wrong method
// on an existing path returns 405 when a MethodNotAllowed handler is
// configured.
func TestConformance_405MethodNotAllowed(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			// Register only GET.
			router.Get("/test", func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			})

			// Register 404 and 405 handlers.
			router.NotFound(func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(404, "text/plain", []byte("not-found"))
			})
			router.MethodNotAllowed(func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(405, "text/plain", []byte("method-not-allowed"))
			})

			// POST to a GET-only route should return 405.
			req := httptest.NewRequest("POST", "/test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			// POST to a GET-only route must return exactly 405 when a
			// MethodNotAllowed handler is configured. All official
			// adapters must have exact 405 parity.
			if resp.StatusCode != 405 {
				t.Fatalf("expected 405, got %d", resp.StatusCode)
			}
		})
	}
}

// TestConformance_405ThroughGroup verifies that 405 dispatch works for
// routes registered through a With-derived group, not just the root
// router. This catches the net/http adapter bug where With-derived
// routers did not share the registered route table with the root.
func TestConformance_405ThroughGroup(t *testing.T) {
	for _, af := range adapterFactories() {
		t.Run(af.Name, func(t *testing.T) {
			router, serve := af.NewRouter()

			// Register GET through a With group with one middleware.
			group := router.With(func(next routing.Handler) routing.Handler {
				return func(ctx *v2wf.RequestContext) error {
					return next(ctx)
				}
			})
			if err := group.Get("/group-test", func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
			}); err != nil {
				t.Fatalf("group.Get: %v", err)
			}

			router.NotFound(func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(404, "text/plain", []byte("not-found"))
			})
			router.MethodNotAllowed(func(ctx *v2wf.RequestContext) error {
				return ctx.Parser.SendResponse(405, "text/plain", []byte("method-not-allowed"))
			})

			// POST to a GET-only group route must return exactly 405.
			req := httptest.NewRequest("POST", "/group-test", nil)
			resp, err := serve(req)
			if err != nil {
				t.Fatalf("serve: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 405 {
				t.Fatalf("expected 405 for group route, got %d", resp.StatusCode)
			}
		})
	}
}
