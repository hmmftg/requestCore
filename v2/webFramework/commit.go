package webFramework

import (
	"net/http"
	"sync"
)

// CommitState tracks whether a response has been committed (status, headers,
// and body have been written) for a given request. All adapters must update
// this state when they write a response so that error dispatch, panic
// recovery, and session middleware can reliably avoid double-writes.
//
// CommitState is safe for concurrent use, though a single request is
// typically processed by one goroutine.
type CommitState struct {
	mu       sync.RWMutex
	committed bool
	status    int
}

// MarkCommitted records that the response has been written with the given
// status code. Subsequent calls are ignored (the first commit wins).
func (c *CommitState) MarkCommitted(status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.committed {
		c.committed = true
		c.status = status
	}
}

// Committed reports whether the response has been committed.
func (c *CommitState) Committed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.committed
}

// Status returns the committed status code, or 0 if not yet committed.
func (c *CommitState) Status() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// BeforeCommitHook is invoked by response methods before the adapter writes
// the response. Hooks may set cookies, persist session state, or record
// metrics. A hook must not write the response body itself.
//
// If a hook returns an error, the response is still committed (the error is
// logged but not propagated to the caller, since the response is about to be
// written). Hooks should be best-effort.
type BeforeCommitHook func(*RequestContext) error

// AddBeforeCommitHook registers a hook to be invoked before the response is
// committed. Hooks fire in registration order. The hook slice is per-request
// and safe to mutate during request processing (not concurrently).
func (c *RequestContext) AddBeforeCommitHook(hook BeforeCommitHook) {
	if hook == nil {
		return
	}
	c.hooksMu.Lock()
	defer c.hooksMu.Unlock()
	c.beforeCommitHooks = append(c.beforeCommitHooks, hook)
}

// RunBeforeCommitHooks invokes all registered before-commit hooks in order.
// Errors are collected but do not abort the commit; the first error is
// returned for logging purposes.
func (c *RequestContext) RunBeforeCommitHooks() error {
	c.hooksMu.RLock()
	hooks := append([]BeforeCommitHook(nil), c.beforeCommitHooks...)
	c.hooksMu.RUnlock()
	var firstErr error
	for _, h := range hooks {
		if err := h(c); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Committed reports whether the response for this context has been committed.
// Adapters update the embedded CommitState when they write a response.
func (c *RequestContext) Committed() bool {
	return c.commit != nil && c.commit.Committed()
}

// MarkCommitted records the committed status. Adapters call this from their
// SendResponse implementation.
func (c *RequestContext) MarkCommitted(status int) {
	if c.commit != nil {
		c.commit.MarkCommitted(status)
	}
}

// CommitState returns the CommitState for this context, or nil if none has
// been associated. Adapters set this when building the RequestContext.
func (c *RequestContext) CommitState() *CommitState {
	return c.commit
}

// SetCommitState associates a CommitState with this context. Adapters call
// this during request setup so that response methods can track commitment.
func (c *RequestContext) SetCommitState(cs *CommitState) {
	c.commit = cs
}

// CookieHelpers is an optional interface that parsers may implement to expose
// cookie setting in a framework-neutral way. The base RequestParser interface
// already includes SetCookie; this is retained for documentation.
type CookieHelpers interface {
	GetCookie(name string) string
	SetCookie(cookie *http.Cookie)
}
