package app

import (
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// DefaultSessionCookieName is the default cookie name for sessions.
const DefaultSessionCookieName = "session"

// SessionMiddleware creates a middleware that loads the session from the
// request cookie and registers a before-commit hook to persist any dirty
// session state back to the response cookie before headers/body are written.
//
// This is a thin wrapper around session.Middleware that also sets the
// Session and Flash fields on the v2 RequestContext for handler access.
// The session is saved via a before-commit hook, not after the handler
// returns, ensuring cookies are set before the response body is written.
func SessionMiddleware(mgr *session.Manager, cookieName string) routing.Middleware {
	if cookieName == "" {
		cookieName = DefaultSessionCookieName
	}
	inner := session.Middleware(mgr, cookieName)
	return func(next routing.Handler) routing.Handler {
		return inner(func(ctx *v2wf.RequestContext) error {
			// Populate ctx.Session and ctx.Flash from the parser locals
			// that session.Middleware set, so handlers can access them
			// directly from the RequestContext.
			ctx.Session = session.FromContext(ctx)
			ctx.Flash = session.FlashFromContext(ctx)
			return next(ctx)
		})
	}
}

// FlashMiddleware creates a middleware that initializes flash messages
// from the session if not already set.
func FlashMiddleware() routing.Middleware {
	return func(next routing.Handler) routing.Handler {
		return func(ctx *v2wf.RequestContext) error {
			if ctx.Flash == nil {
				if ctx.Session != nil {
					if sess, ok := ctx.Session.(*session.Session); ok {
						ctx.Flash = session.LoadFlashFromSession(sess)
					} else {
						ctx.Flash = session.NewFlash()
					}
				} else {
					ctx.Flash = session.NewFlash()
				}
			}

			return next(ctx)
		}
	}
}
