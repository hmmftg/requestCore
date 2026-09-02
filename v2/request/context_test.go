package request

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestContext_Defaults(t *testing.T) {
	ctx := NewContext(context.Background())
	if ctx.Method() != "" {
		t.Fatalf("expected empty method, got %q", ctx.Method())
	}
	if ctx.Path() != "" {
		t.Fatalf("expected empty path, got %q", ctx.Path())
	}
	if ctx.RoutePattern() != "" {
		t.Fatalf("expected empty route pattern, got %q", ctx.RoutePattern())
	}
	if ctx.Response() == nil {
		t.Fatal("expected non-nil response state")
	}
	if ctx.Response().Status() != 200 {
		t.Fatalf("expected default status 200, got %d", ctx.Response().Status())
	}
	if ctx.Principal() != nil {
		t.Fatal("expected nil principal by default")
	}
	if ctx.RequestID() != "" {
		t.Fatal("expected empty request ID by default")
	}
	if ctx.TraceID() != "" {
		t.Fatal("expected empty trace ID by default")
	}
}

func TestContext_Options(t *testing.T) {
	h := make(http.Header)
	h.Set("Authorization", "Bearer token")
	q := url.Values{"page": []string{"1", "2"}}
	params := map[string]string{"id": "42"}
	cookies := []*http.Cookie{{Name: "sess", Value: "abc"}}

	ctx := NewContext(context.Background(),
		WithMethod("POST"),
		WithPath("/users/42"),
		WithRoutePattern("/users/{id}"),
		WithHeader(h),
		WithQuery(q),
		WithPathParams(params),
		WithCookies(cookies),
		WithRemoteAddr("127.0.0.1:1234"),
		WithNative("native-data"),
	)

	if ctx.Method() != "POST" {
		t.Fatalf("expected POST, got %q", ctx.Method())
	}
	if ctx.Path() != "/users/42" {
		t.Fatalf("expected /users/42, got %q", ctx.Path())
	}
	if ctx.RoutePattern() != "/users/{id}" {
		t.Fatalf("expected /users/{id}, got %q", ctx.RoutePattern())
	}
	if ctx.Header("Authorization") != "Bearer token" {
		t.Fatalf("expected Bearer token, got %q", ctx.Header("Authorization"))
	}
	if ctx.Query("page") != "1" {
		t.Fatalf("expected 1, got %q", ctx.Query("page"))
	}
	allPage := ctx.QueryAll("page")
	if len(allPage) != 2 || allPage[0] != "1" || allPage[1] != "2" {
		t.Fatalf("expected [1, 2], got %v", allPage)
	}
	if ctx.PathParam("id") != "42" {
		t.Fatalf("expected 42, got %q", ctx.PathParam("id"))
	}
	c := ctx.Cookie("sess")
	if c == nil || c.Value != "abc" {
		t.Fatalf("expected cookie sess=abc, got %+v", c)
	}
	if ctx.RemoteAddr() != "127.0.0.1:1234" {
		t.Fatalf("expected 127.0.0.1:1234, got %q", ctx.RemoteAddr())
	}
	if ctx.Native() != "native-data" {
		t.Fatalf("expected native-data, got %v", ctx.Native())
	}
}

func TestContext_Context(t *testing.T) {
	goCtx := context.Background()
	ctx := NewContext(goCtx)
	if ctx.Context() == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestContext_NilGoContext(t *testing.T) {
	ctx := NewContext(nil)
	if ctx.Context() == nil {
		t.Fatal("expected background context fallback")
	}
}

func TestContext_WithContext(t *testing.T) {
	ctx := NewContext(context.Background(),
		WithMethod("GET"),
		WithPath("/test"),
		WithRoutePattern("/test"),
	)
	childCtx, cancel := context.WithCancel(ctx.Context())
	defer cancel()
	nc := ctx.WithContext(childCtx)
	if nc.Method() != "GET" {
		t.Fatalf("expected method GET, got %q", nc.Method())
	}
	if nc.Path() != "/test" {
		t.Fatalf("expected path /test, got %q", nc.Path())
	}
	if nc.Context() != childCtx {
		t.Fatal("expected child context")
	}
	// Original should be unchanged.
	if ctx.Context() == childCtx {
		t.Fatal("original context should not be the child")
	}
}

func TestContext_ResponseMutation(t *testing.T) {
	ctx := NewContext(context.Background())
	ctx.Response().SetStatus(201)
	ctx.Response().SetHeader("Location", "/users/1")
	if ctx.Response().Status() != 201 {
		t.Fatalf("expected 201, got %d", ctx.Response().Status())
	}
	if ctx.Response().Header().Get("Location") != "/users/1" {
		t.Fatalf("expected Location /users/1, got %q", ctx.Response().Header().Get("Location"))
	}
}

func TestContext_PrincipalRequestIDTraceID(t *testing.T) {
	ctx := NewContext(context.Background())
	ctx.SetPrincipal("user-123")
	ctx.SetRequestID("req-456")
	ctx.SetTraceID("trace-789")
	if ctx.Principal() != "user-123" {
		t.Fatalf("expected user-123, got %v", ctx.Principal())
	}
	if ctx.RequestID() != "req-456" {
		t.Fatalf("expected req-456, got %q", ctx.RequestID())
	}
	if ctx.TraceID() != "trace-789" {
		t.Fatalf("expected trace-789, got %q", ctx.TraceID())
	}
}

func TestContext_HeadersCopy(t *testing.T) {
	h := make(http.Header)
	h.Set("X-Test", "value")
	ctx := NewContext(context.Background(), WithHeader(h))
	returned := ctx.Headers()
	returned.Set("X-Test", "mutated")
	// Original should be unaffected.
	if ctx.Header("X-Test") != "value" {
		t.Fatalf("expected original value, got %q", ctx.Header("X-Test"))
	}
}

func TestContext_PathParamsCopy(t *testing.T) {
	params := map[string]string{"id": "42"}
	ctx := NewContext(context.Background(), WithPathParams(params))
	returned := ctx.PathParams()
	returned["id"] = "mutated"
	// Original should be unaffected.
	if ctx.PathParam("id") != "42" {
		t.Fatalf("expected original 42, got %q", ctx.PathParam("id"))
	}
}

func TestContext_EmptyAccessors(t *testing.T) {
	ctx := NewContext(context.Background())
	if ctx.Header("X-Test") != "" {
		t.Fatal("expected empty header")
	}
	if ctx.Query("page") != "" {
		t.Fatal("expected empty query")
	}
	if ctx.QueryAll("page") != nil {
		t.Fatal("expected nil queryAll")
	}
	if ctx.PathParam("id") != "" {
		t.Fatal("expected empty path param")
	}
	if len(ctx.PathParams()) != 0 {
		t.Fatal("expected empty path params")
	}
	if ctx.Cookie("sess") != nil {
		t.Fatal("expected nil cookie")
	}
	if len(ctx.Cookies()) != 0 {
		t.Fatal("expected empty cookies")
	}
}

func TestContext_BeforeCommitHooks(t *testing.T) {
	ctx := NewContext(context.Background())
	called := 0
	ctx.AddBeforeCommitHook(func() error {
		called++
		return nil
	})
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected 1 call, got %d", called)
	}
	// Idempotent: second call should not re-run hooks.
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected still 1 call, got %d", called)
	}
}

func TestContext_BeforeCommitHooks_FirstErrorReturned(t *testing.T) {
	ctx := NewContext(context.Background())
	order := []string{}
	ctx.AddBeforeCommitHook(func() error {
		order = append(order, "first")
		return errBoom
	})
	ctx.AddBeforeCommitHook(func() error {
		order = append(order, "second")
		return nil
	})
	err := ctx.RunBeforeCommitHooks()
	if err != errBoom {
		t.Fatalf("expected errBoom, got %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected both hooks to run, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" {
		t.Fatalf("expected [first, second], got %v", order)
	}
}

func TestContext_BeforeCommitHooks_NilIgnored(t *testing.T) {
	ctx := NewContext(context.Background())
	ctx.AddBeforeCommitHook(nil)
	ctx.AddBeforeCommitHook(func() error { return nil })
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

var errBoom = &testError{"boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestTypedKey_SetGet(t *testing.T) {
	ctx := NewContext(context.Background())
	key1 := NewTypedKey()
	key2 := NewTypedKey()

	ctx.Set(key1, "hello")
	ctx.Set(key2, 42)

	v1, ok1 := ctx.Get(key1)
	if !ok1 || v1 != "hello" {
		t.Fatalf("expected key1=hello, got %v ok=%v", v1, ok1)
	}

	v2, ok2 := ctx.Get(key2)
	if !ok2 || v2 != 42 {
		t.Fatalf("expected key2=42, got %v ok=%v", v2, ok2)
	}

	// Different keys should not collide.
	if _, ok := ctx.Get(NewTypedKey()); ok {
		t.Fatal("expected new key to not exist")
	}
}

func TestTypedKey_UniqueIDs(t *testing.T) {
	keys := make(map[uint64]bool)
	for i := 0; i < 100; i++ {
		k := NewTypedKey()
		if keys[k.id] {
			t.Fatalf("duplicate key ID: %d", k.id)
		}
		keys[k.id] = true
	}
}
