// Package httpsemantics provides stdlib-only helpers for HTTP standards
// including OAuth token-response cache headers, RFC 6750 Bearer
// challenges, RFC 9110 conditional requests, RFC 8288 Link headers, and
// RFC 9110 Retry-After parsing/formatting.
//
// These helpers are opt-in and framework-neutral. They do not change
// default behavior of any v2 handler or responder.
package httpsemantics

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ApplyTokenResponseNoStore sets the headers required by RFC 6749 §5.1
// for OAuth 2.0 access-token responses: Cache-Control: no-store and
// Pragma: no-cache. The caller is responsible for determining that the
// response is a token response; this helper only applies the headers.
func ApplyTokenResponseNoStore(h http.Header) {
	h.Set("Cache-Control", "no-store")
	h.Set("Pragma", "no-cache")
}

// BearerError is a standard RFC 6750 Bearer challenge error code.
type BearerError string

const (
	// BearerErrorInvalidToken indicates the access token provided is
	// expired, revoked, malformed, or invalid for other reasons.
	BearerErrorInvalidToken BearerError = "invalid_token"

	// BearerErrorInvalidRequest indicates the request is missing a
	// required parameter, includes an unsupported parameter or
	// parameter value, repeats the same parameter, uses more than one
	// method for including an access token, or is otherwise malformed.
	BearerErrorInvalidRequest BearerError = "invalid_request"

	// BearerErrorInsufficientScope indicates that the access token
	// provided has insufficient scope for the resource being accessed.
	BearerErrorInsufficientScope BearerError = "insufficient_scope"

	// BearerErrorMissingToken indicates that no access token was
	// provided in the request.
	BearerErrorMissingToken BearerError = "missing_token"
)

// BearerChallenge holds the parameters for an RFC 6750 WWW-Authenticate
// Bearer challenge.
type BearerChallenge struct {
	Realm            string
	Error            BearerError
	ErrorDescription string
	ErrorURI         string
	Scope            string
}

// FormatBearerChallenge formats an RFC 6750 WWW-Authenticate Bearer
// challenge string. All parameter values are validated and escaped to
// prevent header injection.
func FormatBearerChallenge(c BearerChallenge) string {
	realm := c.Realm
	if realm == "" {
		realm = "Protected"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf(`realm="%s"`, escapeQuotedString(realm)))

	if c.Error != "" {
		parts = append(parts, fmt.Sprintf(`error="%s"`, escapeQuotedString(string(c.Error))))
	}

	if c.ErrorDescription != "" {
		parts = append(parts, fmt.Sprintf(`error_description="%s"`, escapeQuotedString(c.ErrorDescription)))
	}

	if c.ErrorURI != "" {
		parts = append(parts, fmt.Sprintf(`error_uri="%s"`, escapeQuotedString(c.ErrorURI)))
	}

	if c.Scope != "" {
		parts = append(parts, fmt.Sprintf(`scope="%s"`, escapeQuotedString(c.Scope)))
	}

	return "Bearer " + strings.Join(parts, ", ")
}

// ApplyBearerChallenge sets the WWW-Authenticate header on an
// http.Header with the given Bearer challenge.
func ApplyBearerChallenge(h http.Header, c BearerChallenge) {
	h.Set("WWW-Authenticate", FormatBearerChallenge(c))
}

// escapeQuotedString escapes a string for safe inclusion in an RFC 7235
// quoted-string value.
func escapeQuotedString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidateErrorURI validates that the error_uri is a valid URI reference.
func ValidateErrorURI(uri string) error {
	if uri == "" {
		return nil
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("httpsemantics: invalid error_uri: %w", err)
	}
	if strings.ContainsAny(parsed.String(), "\r\n") {
		return fmt.Errorf("httpsemantics: error_uri contains CRLF")
	}
	return nil
}
