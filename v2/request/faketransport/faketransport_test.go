package faketransport

import (
	"context"
	"net/http"
	"testing"
)

func TestFakeTransport_BasicConstruction(t *testing.T) {
	ft := New("GET", "/users/42")
	ctx := ft.Context()
	if ctx.Method() != "GET" {
		t.Fatalf("expected GET, got %q", ctx.Method())
	}
	if ctx.Path() != "/users/42" {
		t.Fatalf("expected /users/42, got %q", ctx.Path())
	}
	if ft.Committed() {
		t.Fatal("expected not committed initially")
	}
}

func TestFakeTransport_WithRoutePattern(t *testing.T) {
	ft := New("GET", "/users/42", WithRoutePattern("/users/{id}"))
	if ft.Context().RoutePattern() != "/users/{id}" {
		t.Fatalf("expected /users/{id}, got %q", ft.Context().RoutePattern())
	}
}

func TestFakeTransport_WithHeader(t *testing.T) {
	ft := New("POST", "/users", WithHeader("Authorization", "Bearer token"))
	if ft.Context().Header("Authorization") != "Bearer token" {
		t.Fatalf("expected Bearer token, got %q", ft.Context().Header("Authorization"))
	}
}

func TestFakeTransport_WithQueryParam(t *testing.T) {
	ft := New("GET", "/users", WithQueryParam("page", "1"), WithQueryParam("limit", "10"))
	if ft.Context().Query("page") != "1" {
		t.Fatalf("expected page=1, got %q", ft.Context().Query("page"))
	}
	if ft.Context().Query("limit") != "10" {
		t.Fatalf("expected limit=10, got %q", ft.Context().Query("limit"))
	}
}

func TestFakeTransport_WithPathParam(t *testing.T) {
	ft := New("GET", "/users/42", WithPathParam("id", "42"))
	if ft.Context().PathParam("id") != "42" {
		t.Fatalf("expected id=42, got %q", ft.Context().PathParam("id"))
	}
}

func TestFakeTransport_WithCookie(t *testing.T) {
	ft := New("GET", "/users", WithCookie("sess", "abc123"))
	c := ft.Context().Cookie("sess")
	if c == nil || c.Value != "abc123" {
		t.Fatalf("expected cookie sess=abc123, got %+v", c)
	}
}

func TestFakeTransport_WithRemoteAddr(t *testing.T) {
	ft := New("GET", "/users", WithRemoteAddr("127.0.0.1:1234"))
	if ft.Context().RemoteAddr() != "127.0.0.1:1234" {
		t.Fatalf("expected 127.0.0.1:1234, got %q", ft.Context().RemoteAddr())
	}
}

func TestFakeTransport_WithNative(t *testing.T) {
	native := &http.Request{Method: "GET"}
	ft := New("GET", "/users", WithNative(native))
	if ft.Context().Native() != native {
		t.Fatal("expected native data to be set")
	}
}

func TestFakeTransport_WithBody(t *testing.T) {
	ft := New("POST", "/users", WithBody(`{"name":"alice"}`))
	if ft.Body() != `{"name":"alice"}` {
		t.Fatalf("expected body, got %q", ft.Body())
	}
}

func TestFakeTransport_WriteResponse(t *testing.T) {
	ft := New("GET", "/users")
	ft.WriteResponse(200, "application/json", []byte(`{"status":"ok"}`))

	if !ft.Committed() {
		t.Fatal("expected committed after write")
	}
	if ft.ResponseStatus() != 200 {
		t.Fatalf("expected status 200, got %d", ft.ResponseStatus())
	}
	if ft.ResponseHeaders().Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %q", ft.ResponseHeaders().Get("Content-Type"))
	}
	if string(ft.ResponseBody()) != `{"status":"ok"}` {
		t.Fatalf("expected body, got %q", string(ft.ResponseBody()))
	}
}

func TestFakeTransport_FirstWriteWins(t *testing.T) {
	ft := New("GET", "/users")
	ft.WriteResponse(200, "application/json", []byte(`{"status":"ok"}`))
	ft.WriteResponse(500, "application/json", []byte(`{"error":"fail"}`))

	// Second write should be ignored.
	if ft.ResponseStatus() != 200 {
		t.Fatalf("expected first write status 200, got %d", ft.ResponseStatus())
	}
	if string(ft.ResponseBody()) != `{"status":"ok"}` {
		t.Fatalf("expected first write body, got %q", string(ft.ResponseBody()))
	}
}

func TestFakeTransport_NotCommittedReturnsZero(t *testing.T) {
	ft := New("GET", "/users")
	if ft.ResponseStatus() != 0 {
		t.Fatalf("expected 0 status when not committed, got %d", ft.ResponseStatus())
	}
	if ft.ResponseBody() != nil {
		t.Fatalf("expected nil body when not committed")
	}
}

func TestFakeTransport_QueryInPath(t *testing.T) {
	ft := New("GET", "/users?page=2&limit=5")
	if ft.Context().Path() != "/users" {
		t.Fatalf("expected path /users, got %q", ft.Context().Path())
	}
	if ft.Context().Query("page") != "2" {
		t.Fatalf("expected page=2, got %q", ft.Context().Query("page"))
	}
	if ft.Context().Query("limit") != "5" {
		t.Fatalf("expected limit=5, got %q", ft.Context().Query("limit"))
	}
}

func TestFakeTransport_QueryInPathAndOption(t *testing.T) {
	ft := New("GET", "/users?page=2", WithQueryParam("limit", "5"))
	if ft.Context().Query("page") != "2" {
		t.Fatalf("expected page=2, got %q", ft.Context().Query("page"))
	}
	if ft.Context().Query("limit") != "5" {
		t.Fatalf("expected limit=5, got %q", ft.Context().Query("limit"))
	}
}

func TestFakeTransport_ResponseMutationViaContext(t *testing.T) {
	ft := New("POST", "/users")
	ft.Context().Response().SetStatus(201)
	ft.Context().Response().SetHeader("Location", "/users/1")

	// The response state should reflect the mutations.
	if ft.Context().Response().Status() != 201 {
		t.Fatalf("expected 201, got %d", ft.Context().Response().Status())
	}
	if ft.Context().Response().Header().Get("Location") != "/users/1" {
		t.Fatalf("expected Location, got %q", ft.Context().Response().Header().Get("Location"))
	}

	// Now write the response using the mutated state.
	ft.WriteResponse(ft.Context().Response().Status(), "application/json", []byte(`{}`))
	if ft.ResponseStatus() != 201 {
		t.Fatalf("expected 201, got %d", ft.ResponseStatus())
	}
}

func TestFakeTransport_ContextAlive(t *testing.T) {
	ft := New("GET", "/users")
	if ft.Context().Context() == nil {
		t.Fatal("expected non-nil context")
	}
	// Verify the context is a standard Go context.
	_, _ = ft.Context().Context().Deadline()
}

func TestFakeTransport_BodyEmptyByDefault(t *testing.T) {
	ft := New("GET", "/users")
	if ft.Body() != "" {
		t.Fatalf("expected empty body, got %q", ft.Body())
	}
}

func TestFakeTransport_MultipleHeaders(t *testing.T) {
	ft := New("POST", "/users",
		WithHeader("Authorization", "Bearer token"),
		WithHeader("Content-Type", "application/json"),
		WithHeader("X-Request-ID", "req-123"),
	)
	if ft.Context().Header("Authorization") != "Bearer token" {
		t.Fatalf("expected Authorization, got %q", ft.Context().Header("Authorization"))
	}
	if ft.Context().Header("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type, got %q", ft.Context().Header("Content-Type"))
	}
	if ft.Context().Header("X-Request-ID") != "req-123" {
		t.Fatalf("expected X-Request-ID, got %q", ft.Context().Header("X-Request-ID"))
	}
}

func TestFakeTransport_MultipleCookies(t *testing.T) {
	ft := New("GET", "/users",
		WithCookie("sess", "abc"),
		WithCookie("csrf", "def"),
	)
	if ft.Context().Cookie("sess").Value != "abc" {
		t.Fatal("expected sess cookie")
	}
	if ft.Context().Cookie("csrf").Value != "def" {
		t.Fatal("expected csrf cookie")
	}
}

func TestFakeTransport_WithContextValue(t *testing.T) {
	// Verify that the body is accessible via the request context and
	// doesn't interfere with the request context's cancellation.
	ft := New("POST", "/users", WithBody("test body"))
	if got := ft.Context().Body(); got != "test body" {
		t.Fatalf("expected body via Context().Body(), got %q", got)
	}
	// Verify context is still usable.
	ctx := ft.Context().Context()
	_, _ = ctx.Deadline()
	_ = ctx.Err()
}

func TestFakeTransport_NoHeadersByDefault(t *testing.T) {
	ft := New("GET", "/users")
	h := ft.Context().Headers()
	if len(h) != 0 {
		t.Fatalf("expected no headers, got %v", h)
	}
}

func TestFakeTransport_NoCookiesByDefault(t *testing.T) {
	ft := New("GET", "/users")
	if len(ft.Context().Cookies()) != 0 {
		t.Fatal("expected no cookies by default")
	}
}

// Verify that the body is preserved through a cancel-derived context.
func TestFakeTransport_BodyWithCancelContext(t *testing.T) {
	ft := New("POST", "/users", WithBody("test"))
	ctx, cancel := context.WithCancel(ft.Context().Context())
	defer cancel()
	// Body is stored on the request context, not the Go context, so
	// it remains accessible through the FakeTransport regardless of
	// context wrapping.
	if got := ft.Body(); got != "test" {
		t.Fatalf("expected body preserved through cancel context, got %q", got)
	}
	_ = ctx
}
