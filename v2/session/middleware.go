package session

import (
	"log/slog"
	"net/http"

	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/telemetry"
)

// ctxSessionKey is the typed key for storing *Session on a request.Context.
var ctxSessionKey = request.NewTypedKey()

// ctxFlashKey is the typed key for storing *Flash on a request.Context.
var ctxFlashKey = request.NewTypedKey()

// SaveFailureMode controls how the session middleware handles session
// save failures in the before-commit hook.
type SaveFailureMode int

const (
	// SaveStrict (default) propagates save failures to the caller so
	// the response is not committed as a success.
	SaveStrict SaveFailureMode = iota
	// SaveBestEffort logs save failures but does not propagate the
	// error, allowing the response to commit successfully even if the
	// session was not persisted.
	SaveBestEffort
)

// MiddlewareConfig configures the session middleware.
type MiddlewareConfig struct {
	// Manager is the session manager used to load and save sessions.
	Manager *Manager

	// CookieName is the name of the session cookie.
	CookieName string

	// SaveFailureMode controls how session save failures are handled.
	// Default: SaveStrict.
	SaveFailureMode SaveFailureMode

	// Sink is the telemetry sink for logging session load/save
	// outcomes. If nil, no telemetry is emitted.
	Sink telemetry.Sink
}

// Middleware returns a routing.Middleware that loads the session from
// the request cookie and registers a before-commit hook to persist
// any dirty session state back to the response cookie.
//
// The session is stored on the request.Context via the session
// package's typed key and can be retrieved by handlers via
// session.FromContext.
//
// Session save failures are handled in strict mode (default): the
// error is logged via the telemetry sink and propagated to the caller
// so the response is not committed as a success. Use
// MiddlewareWithConfig to select best-effort mode if needed.
func Middleware(manager *Manager, cookieName string) routing.Middleware {
	return MiddlewareWithConfig(MiddlewareConfig{
		Manager:         manager,
		CookieName:      cookieName,
		SaveFailureMode: SaveStrict,
	})
}

// MiddlewareWithConfig returns a routing.Middleware configured by the
// given MiddlewareConfig.
func MiddlewareWithConfig(cfg MiddlewareConfig) routing.Middleware {
	manager := cfg.Manager
	cookieName := cfg.CookieName
	mode := cfg.SaveFailureMode
	sink := cfg.Sink
	return func(next routing.Handler) routing.Handler {
		return func(ctx *request.Context, transport routing.Transport) error {
			// Load session from request cookie.
			var cookieValue string
			if c := ctx.Cookie(cookieName); c != nil {
				cookieValue = c.Value
			}

			sess, flash, err := manager.LoadFromCookie(ctx.Context(), cookieName, cookieValue)
			if err != nil {
				// Emit a security/transaction failure event via
				// telemetry. The raw cookie token is never logged.
				if sink != nil {
					sink.Record(telemetry.Event{
						Type:      telemetry.EventFailure,
						Operation: "session-load",
						Err:       err,
						Attrs: []slog.Attr{
							slog.String("cookie", cookieName),
						},
					})
				}
				// Continue with the fresh session returned by LoadFromCookie.
			}

			// Store session and flash on the request.Context.
			ctx.Set(ctxSessionKey, sess)
			ctx.Set(ctxFlashKey, flash)

			// Register a before-commit hook to persist the session
			// cookie before the response is written.
			ctx.AddBeforeCommitHook(func() error {
				// Persist flash back to session.
				if flash != nil {
					SaveFlashToSession(sess, flash)
				}

				// Only save if the session is dirty.
				if !sess.IsDirty() {
					return nil
				}

				cookie, saveErr := manager.SaveToCookie(ctx.Context(), sess, nil, cookieName)
				if saveErr != nil {
					if sink != nil {
						sink.Record(telemetry.Event{
							Type:      telemetry.EventFailure,
							Operation: "session-save",
							Err:       saveErr,
						})
					}
					if mode == SaveStrict {
						return saveErr
					}
					return nil
				}
				if cookie != nil {
					ctx.Response().AddHeader("Set-Cookie", cookie.String())
				}
				return nil
			})

			return next(ctx, transport)
		}
	}
}

// FromContext retrieves the *Session from a request.Context.
// Returns nil if no session middleware was applied.
func FromContext(ctx *request.Context) *Session {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Get(ctxSessionKey); ok {
		if s, ok := v.(*Session); ok {
			return s
		}
	}
	return nil
}

// FlashFromContext retrieves the *Flash from a request.Context.
// Returns nil if no session middleware was applied.
func FlashFromContext(ctx *request.Context) *Flash {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Get(ctxFlashKey); ok {
		if f, ok := v.(*Flash); ok {
			return f
		}
	}
	return nil
}

// suppress unused import warning.
var _ = http.StatusOK
