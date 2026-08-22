package app

import (
	"context"
	"net/http"

	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/session"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// DefaultSessionCookieName is the default cookie name for sessions.
const DefaultSessionCookieName = "session"

// SessionMiddleware creates a middleware that loads the session from the
// request cookie and stores it in the RequestContext. The session is
// saved back to the store after the handler completes.
//
// The middleware uses the session cookie name specified (default: "session").
// If no session cookie is present, a new session is created.
func SessionMiddleware(mgr *session.Manager, cookieName string) routing.Middleware {
	if cookieName == "" {
		cookieName = DefaultSessionCookieName
	}
	return func(next routing.Handler) routing.Handler {
		return func(ctx *v2wf.RequestContext) error {
			// Get the session cookie from the request
			var cookieValue string

			type cookieGetter interface {
				GetCookie(name string) string
			}
			if cg, ok := ctx.Parser.(cookieGetter); ok {
				cookieValue = cg.GetCookie(cookieName)
			}

			// Load or create session and flash
			sess, flash, err := mgr.LoadFromCookie(ctx.Context, cookieName, cookieValue)
			if err != nil {
				return err
			}
			ctx.Session = sess
			ctx.Flash = flash

			// Run the handler
			err = next(ctx)

			// Save the session and get the cookie to set
			saveCookie, saveErr := mgr.SaveToCookie(context.Background(), sess, flash, cookieName)
			if saveErr != nil {
				if err == nil {
					err = saveErr
				}
			}

			// Set the session cookie if needed
			if saveCookie != nil {
				type cookieSetter interface {
					SetCookie(*http.Cookie)
				}
				if cs, ok := ctx.Parser.(cookieSetter); ok {
					cs.SetCookie(saveCookie)
				}
			}

			return err
		}
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
