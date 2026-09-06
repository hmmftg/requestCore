package remotecall

import (
	"context"
	"encoding/base64"
	"net/http"
)

// AuthProvider applies authentication to request headers before the adapter
// executes the HTTP call. Auth happens in RemoteClient.Call before
// constructing the adapter Request — the adapter has no knowledge of auth.
//
// OAuth2 is NOT part of the core package. It can be implemented as a
// separate package or by the caller. The core package only knows
// AuthProvider.Apply.
//
// Retry attempts reuse the same headers (including auth) — AuthProvider.Apply
// is NOT called per attempt. It is called exactly once per logical Call.
// OAuth2 providers must handle concurrent refresh safely (caller's
// responsibility).
type AuthProvider interface {
	// Apply modifies headers in-place to add authentication.
	Apply(ctx context.Context, headers http.Header) error
}

// BasicAuth returns an AuthProvider that sets HTTP Basic Authentication.
type BasicAuth struct {
	User     string
	Password string
}

// Apply sets the Authorization header for basic auth.
func (b BasicAuth) Apply(_ context.Context, headers http.Header) error {
	headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(b.User+":"+b.Password)))
	return nil
}

// BearerToken returns an AuthProvider that sets a Bearer token.
type BearerToken struct {
	Token string
}

// Apply sets the Authorization header for bearer token auth.
func (b BearerToken) Apply(_ context.Context, headers http.Header) error {
	headers.Set("Authorization", "Bearer "+b.Token)
	return nil
}

// StaticToken returns an AuthProvider that sets a static token with a
// caller-specified token type (e.g., "ApiKey", "JWT").
type StaticToken struct {
	TokenType string
	Token     string
}

// Apply sets the Authorization header for a static token.
func (s StaticToken) Apply(_ context.Context, headers http.Header) error {
	headers.Set("Authorization", s.TokenType+" "+s.Token)
	return nil
}
