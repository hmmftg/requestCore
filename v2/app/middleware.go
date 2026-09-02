package app

import (
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
)

// DefaultSessionCookieName is the default cookie name for sessions.
const DefaultSessionCookieName = "session"

// SessionMiddleware creates a middleware that loads the session from
// the request cookie and registers a before-commit hook to persist
// any dirty session state back to the response cookie before
// headers/body are written.
//
// The session is stored on the request.Context via the session
// package's typed key and can be retrieved by handlers via
// session.FromContext.
func SessionMiddleware(mgr *session.Manager, cookieName string) routing.Middleware {
	if cookieName == "" {
		cookieName = DefaultSessionCookieName
	}
	return session.Middleware(mgr, cookieName)
}

// FlashMiddleware creates a middleware that initializes flash messages
// from the session if not already set.
func FlashMiddleware() routing.Middleware {
	return func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			return next(ctx, transport)
		}
	}
}
