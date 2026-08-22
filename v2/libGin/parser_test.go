package libGin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hmmftg/requestCore/v2/routing"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGinParserV2_SendResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	parser := InitContextV2(c)
	err := parser.SendResponse(200, "application/json", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("SendResponse: %v", err)
	}

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" && ct != "application/json" {
		t.Fatalf("expected application/json content type, got %s", ct)
	}
	if w.Body.String() != `{"ok":true}` {
		t.Fatalf("expected {\"ok\":true}, got %s", w.Body.String())
	}
}

func TestGinParserV2_GetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.AddCookie(&http.Cookie{Name: "session", Value: "abc123"})

	parser := InitContextV2(c)
	val := parser.GetCookie("session")
	if val != "abc123" {
		t.Fatalf("expected 'abc123', got %q", val)
	}

	missing := parser.GetCookie("nonexistent")
	if missing != "" {
		t.Fatalf("expected empty string, got %q", missing)
	}
}

func TestGinParserV2_SetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	parser := InitContextV2(c)
	parser.SetCookie(&http.Cookie{Name: "test", Value: "xyz"})

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "test" || cookies[0].Value != "xyz" {
		t.Fatalf("expected test=xyz, got %s=%s", cookies[0].Name, cookies[0].Value)
	}
}

func TestGinRouter_BasicRoute(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	called := false
	err := router.Get("/users", func(ctx *v2wf.RequestContext) error {
		called = true
		return ctx.Parser.SendResponse(200, "text/plain", []byte("hello"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users", nil)
	engine.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("expected 'hello', got %s", w.Body.String())
	}
}

func TestGinRouter_ParamRoute(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	err := router.Get("/users/{id}", func(ctx *v2wf.RequestContext) error {
		id := ctx.Parser.GetURLParam("id")
		return ctx.Parser.SendResponse(200, "text/plain", []byte(id))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/123", nil)
	engine.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "123" {
		t.Fatalf("expected '123', got %s", w.Body.String())
	}
}

func TestGinRouter_Group(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	api := router.Group("/api")
	called := false
	err := api.Get("/users", func(ctx *v2wf.RequestContext) error {
		called = true
		return ctx.Parser.SendResponse(200, "text/plain", []byte("ok"))
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users", nil)
	engine.ServeHTTP(w, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGinRouter_Middleware(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

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

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

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

func TestGinRouter_NotFound(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	router.NotFound(func(ctx *v2wf.RequestContext) error {
		return ctx.Parser.SendResponse(404, "text/plain", []byte("not found"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	engine.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if w.Body.String() != "not found" {
		t.Fatalf("expected 'not found', got %s", w.Body.String())
	}
}

func TestGinRouter_InvalidPattern(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	err := router.Get("/users/{id", func(ctx *v2wf.RequestContext) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestGinRouter_Native(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	if router.Native() != engine {
		t.Fatal("expected Native() to return the gin.Engine")
	}
}

func TestGinRouter_HandlerError(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	router.Get("/fail", func(ctx *v2wf.RequestContext) error {
		ctx2, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx2.Err()
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	engine.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGinRouter_MethodNotAllowed(t *testing.T) {
	engine := gin.New()
	router := NewRouter(engine)

	router.MethodNotAllowed(func(ctx *v2wf.RequestContext) error {
		return ctx.Parser.SendResponse(http.StatusMethodNotAllowed, "text/plain", []byte("method not allowed"))
	})

	router.Post("/items", func(ctx *v2wf.RequestContext) error {
		return ctx.Parser.SendResponse(http.StatusOK, "text/plain", []byte("created"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/items", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGinRouter_NewRouterFromGroup_NotFoundPanics(t *testing.T) {
	group := &gin.RouterGroup{}
	router := NewRouterFromGroup(group)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for NotFound on group-only router")
		}
	}()
	router.NotFound(func(ctx *v2wf.RequestContext) error {
		return nil
	})
}
