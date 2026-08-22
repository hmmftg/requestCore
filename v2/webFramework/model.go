// Package webFramework provides the v2 web framework abstraction layer.
//
// It extends the root module's [github.com/hmmftg/requestCore/webFramework]
// with renderer support, cookie access, and session/flash integration.
// Framework adapters (libGin, libFiber, libNetHttp) implement these
// interfaces for their respective HTTP frameworks.
package webFramework

import (
	"context"
	"net/http"
	"sync"

	legacy "github.com/hmmftg/requestCore/webFramework"
)

// RequestParser extends the v1 RequestParser with raw response writing,
// cookie access, and session/flash support.
//
// Renderers produce encoded bytes; the parser's SendResponse method
// writes them to the framework-specific transport. This avoids leaking
// net/http types (like http.ResponseWriter) into Fiber/fasthttp adapters.
//
// All SendResponse implementations must check and update the bound
// CommitState so that error dispatch, panic recovery, and session
// middleware can reliably avoid double-writes across v2 and legacy
// response paths.
type RequestParser interface {
	legacy.RequestParser

	// SendResponse writes a raw response with the given status code,
	// content type, and body bytes. The adapter handles framework-specific
	// transport mechanics (gin.Context.Writer, fiber.Ctx, http.ResponseWriter).
	//
	// If a CommitState has been bound via SetCommitState and the response
	// is already committed, this method returns nil without writing.
	// On successful write, the commit state is marked committed.
	SendResponse(status int, contentType string, body []byte) error

	// GetCookie returns the value of the named request cookie.
	// Returns "" if the cookie does not exist.
	GetCookie(name string) string

	// SetCookie sets an HTTP response cookie.
	SetCookie(cookie *http.Cookie)

	// SetCommitState binds the request's CommitState to this parser so
	// that SendResponse can check and update the committed status.
	// Adapters call this during request setup, after creating the parser
	// and before running handlers.
	SetCommitState(cs *CommitState)

	// SetBeforeCommitHookRunner binds a function that runs before-commit
	// hooks before the response is written. SendResponse implementations
	// must call this function (if non-nil) before writing the response,
	// so that direct writes (not going through response.Handler.commit)
	// also execute hooks such as session cookie persistence.
	//
	// The function is idempotent: repeated calls return nil after the
	// first invocation. Hook errors are logged inside the runner and do
	// not block the write.
	SetBeforeCommitHookRunner(fn func() error)
}

// RequestContext holds the per-request state passed through the v2
// handler and middleware pipeline.
type RequestContext struct {
	// Context is the v2 request context (may carry cancellation, tracing).
	Context context.Context

	// LegacyContext is the framework-native context expected by
	// libContext.InitContext (e.g. *gin.Context, *fiber.Ctx, context.Context
	// with net/http request/response). Used by the legacy handler adapter.
	// It is typed as any because *fiber.Ctx does not implement
	// context.Context but is a valid input to libContext.InitContext.
	LegacyContext any

	// Parser is the v2 request parser.
	Parser RequestParser

	// Legacy is the v1 WebFramework, providing access to existing
	// query, persistence, response, logging, and tracing infrastructure.
	// Its Parser field is the same object as Parser above (type-asserted
	// to the legacy RequestParser interface).
	Legacy legacy.WebFramework

	// Session is the per-request session, loaded by session middleware.
	// May be nil if session middleware has not run. Typed as any to
	// avoid an import cycle with the session package; callers should
	// type-assert to *session.Session.
	Session any

	// Flash is the per-request flash, loaded by session middleware.
	// May be nil if session middleware has not run. Typed as any to
	// avoid an import cycle with the session package; callers should
	// type-assert to *session.Flash.
	Flash any

	// commit tracks whether the response has been written. Adapters
	// update this from SendResponse so error dispatch and panic recovery
	// can avoid double-writes.
	commit *CommitState

	// beforeCommitHooks are invoked before the response is committed.
	// Session middleware uses this to persist cookies before the body
	// is written.
	hooksMu           sync.RWMutex
	beforeCommitHooks []BeforeCommitHook

	// hooksRan ensures RunBeforeCommitHooks executes hooks exactly once
	// per request, whether called from the parser's SendResponse or from
	// response.Handler.commit.
	hooksRan bool
}

// LegacyWebFramework returns the v1 WebFramework for use with existing
// query, persistence, response, and API-call helpers.
func (c *RequestContext) LegacyWebFramework() legacy.WebFramework {
	return c.Legacy
}

// WebFrameworkV2 is a convenience wrapper that bundles a RequestContext
// with its parser and legacy framework. It is used by response handlers
// and error handlers that need both v1 and v2 access.
type WebFrameworkV2 struct {
	RequestContext
}
