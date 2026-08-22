package session

import (
	"log/slog"

	"github.com/hmmftg/requestCore/webFramework"

	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// SessionKey is the local storage key for the *Session on the request parser.
const SessionKey = "_v2_session"

// FlashKey is the local storage key for the *Flash on the request parser.
const FlashKey = "_v2_flash"

// Middleware returns a v2 middleware that loads the session from the
// request cookie and registers a before-commit hook to persist any
// dirty session state back to the response cookie.
//
// The middleware stores the *Session and *Flash on the request parser's
// locals (SessionKey and FlashKey) so handlers can access them via
// FromContext.
func Middleware(manager *Manager, cookieName string) func(next func(*v2wf.RequestContext) error) func(*v2wf.RequestContext) error {
	return func(next func(*v2wf.RequestContext) error) func(*v2wf.RequestContext) error {
		return func(ctx *v2wf.RequestContext) error {
			// Load session from request cookie.
			var cookieValue string
			if ctx.Parser != nil {
				cookieValue = ctx.Parser.GetCookie(cookieName)
			}

			sess, flash, err := manager.LoadFromCookie(ctx.Context, cookieName, cookieValue)
			if err != nil {
				// On load error, create a fresh session.
				sess = NewSession(manager.Store())
				flash = NewFlash()
			}

			// Store session and flash on both the parser locals (for
			// FromContext/FlashFromContext) and the RequestContext fields
			// (for direct access via ctx.Session/ctx.Flash).
			ctx.Session = sess
			ctx.Flash = flash
			if ctx.Parser != nil {
				ctx.Parser.SetLocal(SessionKey, sess)
				ctx.Parser.SetLocal(FlashKey, flash)
			}

			// Register a before-commit hook to persist the session
			// cookie before the response is written. This ensures
			// that session changes are saved even if the handler
			// doesn't explicitly call Save.
			ctx.AddBeforeCommitHook(func(c *v2wf.RequestContext) error {
				// Persist flash back to session.
				if flash != nil {
					SaveFlashToSession(sess, flash)
				}

				// Only save if the session is dirty.
				if !sess.IsDirty() {
					return nil
				}

				cookie, err := manager.SaveToCookie(c.Context, sess, nil, cookieName)
				if err != nil {
					// Log the error but don't block the response.
					if c.Legacy.Parser != nil {
						w := webFramework.WebFramework{Parser: c.Legacy.Parser}
						webFramework.AddLog(w, "session-save-failed",
							slog.Any("error", err))
					}
					return nil
				}
				if cookie != nil && c.Parser != nil {
					c.Parser.SetCookie(cookie)
				}
				return nil
			})

			return next(ctx)
		}
	}
}

// FromContext retrieves the *Session from the request context's parser locals.
// Returns nil if no session middleware was applied.
func FromContext(ctx *v2wf.RequestContext) *Session {
	if ctx == nil || ctx.Parser == nil {
		return nil
	}
	if v := ctx.Parser.GetLocal(SessionKey); v != nil {
		if s, ok := v.(*Session); ok {
			return s
		}
	}
	return nil
}

// FlashFromContext retrieves the *Flash from the request context's parser locals.
// Returns nil if no session middleware was applied.
func FlashFromContext(ctx *v2wf.RequestContext) *Flash {
	if ctx == nil || ctx.Parser == nil {
		return nil
	}
	if v := ctx.Parser.GetLocal(FlashKey); v != nil {
		if f, ok := v.(*Flash); ok {
			return f
		}
	}
	return nil
}
