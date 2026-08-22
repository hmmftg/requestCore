package libFiber

import (
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

func TestFiberParserV2_SendResponse(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		parser := InitContextV2(c)
		return parser.SendResponse(200, "application/json", []byte(`{"ok":true}`))
	})

	resp, _ := app.Test(httptest.NewRequest("GET", "/test", nil))
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Fatalf("expected {\"ok\":true}, got %s", string(body))
	}
}

func TestFiberParserV2_GetCookie(t *testing.T) {
	app := fiber.New()
	app.Get("/test", func(c *fiber.Ctx) error {
		parser := InitContextV2(c)
		val := parser.GetCookie("session")
		return c.SendString(val)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Cookie", "session=abc123")
	resp, _ := app.Test(req)
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "abc123" {
		t.Fatalf("expected 'abc123', got %s", string(body))
	}
}

func TestFiberRouter_BasicRoute(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	called := false
	err := router.Get("/users", func(ctx *v2wf.RequestContext) error {
		called = true
		return ctx.Parser.SendResponse(200, "text/plain", []byte("hello"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	resp, _ := app.Test(httptest.NewRequest("GET", "/users", nil))
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

func TestFiberRouter_ParamRoute(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	err := router.Get("/users/{id}", func(ctx *v2wf.RequestContext) error {
		id := ctx.Parser.GetURLParam("id")
		return ctx.Parser.SendResponse(200, "text/plain", []byte(id))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	resp, _ := app.Test(httptest.NewRequest("GET", "/users/123", nil))
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "123" {
		t.Fatalf("expected '123', got %s", string(body))
	}
}

func TestFiberRouter_Group(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	api := router.Group("/api")
	called := false
	err := api.Get("/users", func(ctx *v2wf.RequestContext) error {
		called = true
		return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	resp, _ := app.Test(httptest.NewRequest("GET", "/api/users", nil))
	if !called {
		t.Fatal("expected handler to be called")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFiberRouter_Middleware(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

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

	resp, _ := app.Test(httptest.NewRequest("GET", "/test", nil))
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
}

func TestFiberRouter_InvalidPattern(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	err := router.Get("/users/{id", func(ctx *v2wf.RequestContext) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestFiberRouter_Native(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)
	if router.Native() != app {
		t.Fatal("expected Native() to return the fiber.App")
	}
}

func TestFiberRouter_HandlerError(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	router.Get("/fail", func(ctx *v2wf.RequestContext) error {
		ctx2, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx2.Err()
	})

	resp, _ := app.Test(httptest.NewRequest("GET", "/fail", nil))
	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestGetFiberCtx(t *testing.T) {
	app := fiber.New()
	router := NewRouter(app)

	var extracted *fiber.Ctx
	router.Get("/test", func(ctx *v2wf.RequestContext) error {
		extracted = GetFiberCtx(ctx.LegacyContext)
		return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
	})

	app.Test(httptest.NewRequest("GET", "/test", nil))
	if extracted == nil {
		t.Fatal("expected non-nil fiber.Ctx from GetFiberCtx")
	}
}
