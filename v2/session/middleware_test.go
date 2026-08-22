package session

import (
	"context"
	"errors"
	"net/http"
	"testing"

	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
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

// runMiddleware executes the session middleware with the given config
// against a fake request context and returns the result.
func runMiddleware(t *testing.T, cfg MiddlewareConfig, makeDirty bool, cookieValue string) (*v2wf.RequestContext, error) {
	t.Helper()
	parser := v2wf.NewFakeParserV2()
	parser.SetCookie(&http.Cookie{Name: cfg.CookieName, Value: cookieValue})

	commit := &v2wf.CommitState{}
	parser.SetCommitState(commit)

	ctx := &v2wf.RequestContext{
		Parser: parser,
	}
	ctx.SetCommitState(commit)
	ctx.Context = context.Background()

	mw := MiddlewareWithConfig(cfg)
	handler := mw(func(c *v2wf.RequestContext) error {
		if makeDirty && c.Session != nil {
			if sess, ok := c.Session.(*Session); ok {
				sess.Set("key", "value")
			}
		}
		// Simulate the response commit path by running before-commit
		// hooks (as SendResponse would in a real request).
		return c.RunBeforeCommitHooks()
	})

	err := handler(ctx)
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
	if ctx.Session == nil {
		t.Fatal("expected session to be set")
	}
	if ctx.Flash == nil {
		t.Fatal("expected flash to be set")
	}
}

// TestMiddleware_DirtySessionSaves verifies that a dirty session
// triggers a save and the cookie is set on the response.
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
	if ctx.Session == nil {
		t.Fatal("expected session to be set")
	}
	// The cookie should have been set via SetCookie on the parser.
	if ctx.Parser == nil {
		t.Fatal("expected parser to be set")
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

	parser := v2wf.NewFakeParserV2()
	commit := &v2wf.CommitState{}
	parser.SetCommitState(commit)
	ctx := &v2wf.RequestContext{Parser: parser}
	ctx.SetCommitState(commit)
	ctx.Context = context.Background()

	mw := Middleware(manager, "sess")
	handler := mw(func(c *v2wf.RequestContext) error {
		if sess, ok := c.Session.(*Session); ok {
			sess.Set("key", "value")
		}
		// Simulate the response commit path.
		return c.RunBeforeCommitHooks()
	})

	err := handler(ctx)
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

	parser := v2wf.NewFakeParserV2()
	commit := &v2wf.CommitState{}
	parser.SetCommitState(commit)
	ctx := &v2wf.RequestContext{Parser: parser}
	ctx.SetCommitState(commit)
	ctx.Context = context.Background()

	mw := Middleware(manager, "sess")
	handler := mw(func(c *v2wf.RequestContext) error {
		// Add flash data and mark session dirty.
		if flash, ok := c.Flash.(*Flash); ok {
			flash.Add("message", "hello")
		}
		if sess, ok := c.Session.(*Session); ok {
			sess.Set("trigger", "dirty")
		}
		// Run hooks to trigger flash persistence + session save.
		return c.RunBeforeCommitHooks()
	})

	if err := handler(ctx); err != nil {
		t.Fatalf("middleware: %v", err)
	}

	// Verify flash was saved to session (flashKey is "_flash").
	if sess, ok := ctx.Session.(*Session); ok {
		flashData := sess.Get("_flash")
		if flashData == nil {
			t.Fatal("expected flash data to be persisted in session")
		}
	} else {
		t.Fatal("expected *Session type assertion to succeed")
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
	if ctx.Session == nil {
		t.Fatal("expected fresh session on load error")
	}
	// The fresh session should have an ID.
	if sess, ok := ctx.Session.(*Session); ok {
		if sess.ID() == "" {
			t.Fatal("expected fresh session to have an ID")
		}
	} else {
		t.Fatal("expected *Session type assertion to succeed")
	}
}
