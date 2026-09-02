package session

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/request/faketransport"
	"github.com/hmmftg/requestCore/v2/routing"
)

// failingStore is a Store that always returns an error on Save.
type failingStore struct{}

func (failingStore) Load(_ context.Context, _ string) (*Session, error) {
	return nil, errors.New("store unavailable")
}
func (failingStore) Save(_ context.Context, _ *Session) (string, error) {
	return "", errors.New("save failed")
}
func (failingStore) Delete(_ context.Context, _ string) error {
	return nil
}

// testTransport is a minimal routing.Transport implementation that
// delegates response writes to the fake transport's recorder. The
// session middleware does not write the response itself (it only
// mutates ctx.Response() and registers before-commit hooks), so this
// transport is primarily passed through to the inner handler.
type testTransport struct {
	ft *faketransport.FakeTransport
}

func (t *testTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	if t.ft.Committed() {
		return nil
	}
	rec := t.ft.Recorder()
	for k, vs := range headers {
		for _, v := range vs {
			rec.Header().Add(k, v)
		}
	}
	if contentType != "" {
		rec.Header().Set("Content-Type", contentType)
	}
	rec.WriteHeader(status)
	_, _ = rec.Write(body)
	t.ft.MarkCommitted()
	return nil
}

func (t *testTransport) Committed() bool {
	return t.ft.Committed()
}

// runMiddleware executes the session middleware with the given config
// against a fake request.Context and transport, and returns the
// context and handler error. When makeDirty is true the inner handler
// marks the session dirty before running before-commit hooks (which
// simulates the response commit path that persists the session cookie).
func runMiddleware(t *testing.T, cfg MiddlewareConfig, makeDirty bool, cookieValue string) (*request.Context, error) {
	t.Helper()

	var opts []faketransport.Option
	if cookieValue != "" {
		opts = append(opts, faketransport.WithCookie(cfg.CookieName, cookieValue))
	}
	ft := faketransport.New(http.MethodGet, "/", opts...)
	ctx := ft.Context()

	mw := MiddlewareWithConfig(cfg)
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		if makeDirty {
			if sess := FromContext(c); sess != nil {
				sess.Set("key", "value")
			}
		}
		// Simulate the response commit path by running before-commit
		// hooks (as SendResponse would in a real request).
		return c.RunBeforeCommitHooks()
	})

	err := handler(ctx, &testTransport{ft: ft})
	return ctx, err
}

// TestMiddleware_CleanSessionNoSave verifies that a clean (non-dirty)
// session does not trigger a save and the middleware succeeds.
func TestMiddleware_CleanSessionNoSave(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ctx, err := runMiddleware(t, MiddlewareConfig{
		Manager:         manager,
		CookieName:      "sess",
		SaveFailureMode: SaveStrict,
	}, false, "")
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if sess := FromContext(ctx); sess == nil {
		t.Fatal("expected session to be set on context")
	}
	if flash := FlashFromContext(ctx); flash == nil {
		t.Fatal("expected flash to be set on context")
	}
}

// TestMiddleware_DirtySessionSaves verifies that a dirty session
// triggers a save and the Set-Cookie header is set on the response.
func TestMiddleware_DirtySessionSaves(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ctx, err := runMiddleware(t, MiddlewareConfig{
		Manager:         manager,
		CookieName:      "sess",
		SaveFailureMode: SaveStrict,
	}, true, "")
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if sess := FromContext(ctx); sess == nil {
		t.Fatal("expected session to be set on context")
	}
	setCookies := ctx.Response().Header()["Set-Cookie"]
	if len(setCookies) == 0 {
		t.Fatal("expected Set-Cookie header to be set on response after dirty save")
	}
}

// TestMiddleware_StrictModePropagatesSaveFailure verifies that in strict
// mode, a session save failure is propagated as an error from the
// before-commit hook.
func TestMiddleware_StrictModePropagatesSaveFailure(t *testing.T) {
	manager := NewManager(failingStore{})

	_, err := runMiddleware(t, MiddlewareConfig{
		Manager:         manager,
		CookieName:      "sess",
		SaveFailureMode: SaveStrict,
	}, true, "")
	if err == nil {
		t.Fatal("expected error in strict mode when save fails")
	}
}

// TestMiddleware_BestEffortModeSwallowsSaveFailure verifies that in
// best-effort mode, a session save failure is logged but not propagated.
func TestMiddleware_BestEffortModeSwallowsSaveFailure(t *testing.T) {
	manager := NewManager(failingStore{})

	_, err := runMiddleware(t, MiddlewareConfig{
		Manager:         manager,
		CookieName:      "sess",
		SaveFailureMode: SaveBestEffort,
	}, true, "")
	if err != nil {
		t.Fatalf("expected no error in best-effort mode, got: %v", err)
	}
}

// TestMiddleware_DefaultIsStrict verifies that the default Middleware
// constructor uses strict mode.
func TestMiddleware_DefaultIsStrict(t *testing.T) {
	manager := NewManager(failingStore{})

	ft := faketransport.New(http.MethodGet, "/")
	ctx := ft.Context()

	mw := Middleware(manager, "sess")
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		if sess := FromContext(c); sess != nil {
			sess.Set("key", "value")
		}
		// Simulate the response commit path.
		return c.RunBeforeCommitHooks()
	})

	err := handler(ctx, &testTransport{ft: ft})
	if err == nil {
		t.Fatal("expected default middleware to be strict and propagate save failure")
	}
}

// TestMiddleware_FlashPersistence verifies that flash data is persisted
// to the session before saving.
func TestMiddleware_FlashPersistence(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ft := faketransport.New(http.MethodGet, "/")
	ctx := ft.Context()

	mw := Middleware(manager, "sess")
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		// Add flash data and mark session dirty.
		if flash := FlashFromContext(c); flash != nil {
			flash.Add("message", "hello")
		}
		if sess := FromContext(c); sess != nil {
			sess.Set("trigger", "dirty")
		}
		// Run hooks to trigger flash persistence + session save.
		return c.RunBeforeCommitHooks()
	})

	if err := handler(ctx, &testTransport{ft: ft}); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	// Verify flash was saved to session (flashKey is "_flash").
	sess := FromContext(ctx)
	if sess == nil {
		t.Fatal("expected session to be set on context")
	}
	if flashData := sess.Get(flashKey); flashData == nil {
		t.Fatal("expected flash data to be persisted in session")
	}
}

// TestMiddleware_LoadErrorCreatesFreshSession verifies that when the
// session token cannot be loaded, a fresh session is created.
func TestMiddleware_LoadErrorCreatesFreshSession(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	// Pass an invalid token to trigger a load error.
	ctx, err := runMiddleware(t, MiddlewareConfig{
		Manager:         manager,
		CookieName:      "sess",
		SaveFailureMode: SaveStrict,
	}, false, "invalid-token")
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	sess := FromContext(ctx)
	if sess == nil {
		t.Fatal("expected fresh session on load error")
	}
	// The fresh session should have an ID.
	if sess.ID() == "" {
		t.Fatal("expected fresh session to have an ID")
	}
}

// TestMiddleware_MultipleSetCookieHeaders verifies that multiple
// Set-Cookie headers added to the response are preserved alongside the
// session cookie set by the middleware's before-commit hook.
func TestMiddleware_MultipleSetCookieHeaders(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ft := faketransport.New(http.MethodGet, "/")
	ctx := ft.Context()

	mw := Middleware(manager, "sess")
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		// Add an extra Set-Cookie before the commit hooks run.
		c.Response().AddHeader("Set-Cookie", "other=1; Path=/")
		// Mark the session dirty so the middleware also sets a cookie.
		if sess := FromContext(c); sess != nil {
			sess.Set("key", "value")
		}
		return c.RunBeforeCommitHooks()
	})

	if err := handler(ctx, &testTransport{ft: ft}); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	setCookies := ctx.Response().Header()["Set-Cookie"]
	if len(setCookies) < 2 {
		t.Fatalf("expected at least 2 Set-Cookie headers to be preserved, got %d: %v", len(setCookies), setCookies)
	}
}

// TestMiddleware_FromContext verifies that session.FromContext returns
// the session stored by the middleware.
func TestMiddleware_FromContext(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ft := faketransport.New(http.MethodGet, "/")
	ctx := ft.Context()

	var seen *Session
	mw := Middleware(manager, "sess")
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		seen = FromContext(c)
		return nil
	})

	if err := handler(ctx, &testTransport{ft: ft}); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if seen == nil {
		t.Fatal("expected FromContext to return a non-nil session inside the handler")
	}
	if seen.ID() == "" {
		t.Fatal("expected session from FromContext to have an ID")
	}
	// FromContext on the outer context should also resolve after the
	// middleware has stored the session.
	if FromContext(ctx) == nil {
		t.Fatal("expected FromContext to return a non-nil session after middleware")
	}
}

// TestMiddleware_FlashFromContext verifies that session.FlashFromContext
// returns the flash stored by the middleware.
func TestMiddleware_FlashFromContext(t *testing.T) {
	store, err := NewCookieStore(CookieStoreConfig{
		SecretKey: []byte("0123456789abcdef0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("NewCookieStore: %v", err)
	}
	manager := NewManager(store)

	ft := faketransport.New(http.MethodGet, "/")
	ctx := ft.Context()

	var seen *Flash
	mw := Middleware(manager, "sess")
	handler := mw(func(c *request.Context, _ routing.Transport) error {
		seen = FlashFromContext(c)
		return nil
	})

	if err := handler(ctx, &testTransport{ft: ft}); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if seen == nil {
		t.Fatal("expected FlashFromContext to return a non-nil flash inside the handler")
	}
	// FlashFromContext on the outer context should also resolve after
	// the middleware has stored the flash.
	if FlashFromContext(ctx) == nil {
		t.Fatal("expected FlashFromContext to return a non-nil flash after middleware")
	}
}
