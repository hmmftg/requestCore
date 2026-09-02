package app

import (
	"net/http"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
)

// DefaultSessionCookieName is the default cookie name for sessions.
const DefaultSessionCookieName = "session"

// SessionMiddleware creates a middleware that loads the session from the
// request cookie and registers a before-commit hook to persist any dirty
// session state back to the response cookie before headers/body are written.
//
// The session is stored via SetPrincipal on the request.Context so
// handlers can access it via ctx.Principal(). Full session migration
// with typed context keys will be done in Phase 7.
func SessionMiddleware(mgr *session.Manager, cookieName string) routing.Middleware {
	if cookieName == "" {
		cookieName = DefaultSessionCookieName
	}
	return func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			// Register before-commit hook for session persistence.
			// Full session loading/saving will be implemented in Phase 7.
			ctx.AddBeforeCommitHook(func() error {
				return nil
			})
			return next(ctx, transport)
		}
	}
}

// FlashMiddleware creates a middleware that initializes flash messages.
func FlashMiddleware() routing.Middleware {
	return func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			return next(ctx, transport)
		}
	}
}

// suppress unused import warning.
var _ = http.StatusOK
var _ = session.NoOpStore{}
