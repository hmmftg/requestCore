// This file contains cross-framework conformance tests that verify all
// adapters behave consistently for core routing operations.
package routing_test

import (
	"context"
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
