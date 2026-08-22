package session

import (
	"context"
	"net/http"
)

// Manager coordinates session and flash lifecycle across HTTP requests.
// It is initialized with a Store and provides methods for loading
// sessions from request cookies and saving them to response cookies.
type Manager struct {
	store Store
}

// NewManager creates a session Manager backed by the given Store.
func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

// Store returns the underlying session store.
func (m *Manager) Store() Store {
	return m.store
}

// LoadFromCookie extracts the session token from the named cookie
// and loads the session from the store.
// If the cookie is absent, a new empty session is returned.
func (m *Manager) LoadFromCookie(ctx context.Context, cookieName, cookieValue string) (*Session, *Flash, error) {
	if cookieValue == "" {
		sess := NewSession(m.store)
		return sess, NewFlash(), nil
	}

	sess, err := m.store.Load(ctx, cookieValue)
	if err != nil {
		sess = NewSession(m.store)
		return sess, NewFlash(), nil
	}

	flash := LoadFlashFromSession(sess)
	return sess, flash, nil
}

// SaveToCookie persists the session and returns an http.Cookie to set
// on the response. If the session is not dirty, the cookie is not
// modified (returns nil).
func (m *Manager) SaveToCookie(ctx context.Context, sess *Session, flash *Flash, cookieName string) (*http.Cookie, error) {
	if flash != nil {
		SaveFlashToSession(sess, flash)
	}

	if !sess.IsDirty() {
		return nil, nil
	}

	token, err := sess.Save(ctx)
	if err != nil {
		return nil, err
	}

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	if cs, ok := m.store.(*CookieStore); ok {
		cfg := cs.Config()
		cookie.Path = cfg.Path
		cookie.Domain = cfg.Domain
		cookie.Secure = cfg.Secure
		cookie.HttpOnly = *cfg.HttpOnly
		cookie.MaxAge = int(cfg.MaxAge.Seconds())

		switch cfg.SameSite {
		case "strict":
			cookie.SameSite = http.SameSiteStrictMode
		case "none":
			cookie.SameSite = http.SameSiteNoneMode
		default:
			cookie.SameSite = http.SameSiteLaxMode
		}
	}

	return cookie, nil
}

// ExpireCookie returns a cookie that immediately expires the session cookie.
func (m *Manager) ExpireCookie(cookieName string) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
