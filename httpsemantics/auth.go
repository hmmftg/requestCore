// Package httpsemantics provides stdlib-only helpers for HTTP standards
// including OAuth token-response cache headers, RFC 6750 Bearer
// challenges, RFC 9110 conditional requests, RFC 8288 Link headers, and
// RFC 9110 Retry-After parsing/formatting.
//
// These helpers are opt-in and framework-neutral. They do not change
// default behavior of any requestCore handler or responder.
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
//
// RFC 6749 §5.1: "The authorization server MUST include the HTTP
// "Cache-Control" response header field [RFC2616] with a value of
// "no-store" in any successful response to the token endpoint. The
// authorization server MUST include the "Pragma" response header field
// [RFC2616] with a value of "no-cache" in any successful response to
// the token endpoint."
func ApplyTokenResponseNoStore(h http.Header) {
	h.Set("Cache-Control", "no-store")
	h.Set("Pragma", "no-cache")
}

// ApplyTokenResponseNoStoreToParser sets the no-store headers through a
// SetHeader function (e.g. webFramework.RequestParser.SetRespHeader).
// This is the v1 integration point for framework-neutral header setting.
func ApplyTokenResponseNoStoreToParser(setHeader func(name, value string)) {
	setHeader("Cache-Control", "no-store")
	setHeader("Pragma", "no-cache")
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
	// provided in the request. This is not a standard RFC 6750 error
	// code but is commonly used in WWW-Authenticate challenges to
	// distinguish "no token" from "invalid token".
	BearerErrorMissingToken BearerError = "missing_token"
)

// BearerChallenge holds the parameters for an RFC 6750 WWW-Authenticate
// Bearer challenge. All fields are validated and escaped by
// FormatBearerChallenge before being included in the header value.
type BearerChallenge struct {
	// Realm is a description of the protected resource. If empty,
	// "Protected" is used as a default.
	Realm string

	// Error is the RFC 6750 error code. If empty, no error parameter is
	// included (used for initial 401 challenges without a specific error).
	Error BearerError

	// ErrorDescription is a human-readable description of the error.
	// Must not contain characters that require escaping beyond standard
	// quoted-string rules. If empty, the parameter is omitted.
	ErrorDescription string

	// ErrorURI is a URI identifying a human-readable page with
	// information about the error. Must be a valid absolute or relative
	// URI. If empty, the parameter is omitted.
	ErrorURI string

	// Scope is a space-delimited list of scopes that would suffice for
	// the request. If empty, the parameter is omitted.
	Scope string
}

// FormatBearerChallenge formats an RFC 6750 WWW-Authenticate Bearer
// challenge string. All parameter values are validated and escaped to
// prevent header injection. The realm is always included; other
// parameters are included only when non-empty.
//
// The returned string is suitable for setting as the WWW-Authenticate
// response header value on a 401 response from a Bearer-protected
// resource server.
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
// http.Header with the given Bearer challenge. The status code should
// be 401 (Unauthorized). The caller is responsible for determining that
// the response is a Bearer-protected resource-server 401; this helper
// does not automatically attach challenges to unrelated 401 responses.
func ApplyBearerChallenge(h http.Header, c BearerChallenge) {
	h.Set("WWW-Authenticate", FormatBearerChallenge(c))
}

// ApplyBearerChallengeToParser sets the WWW-Authenticate header through
// a SetHeader function (e.g. webFramework.RequestParser.SetRespHeader).
// This is the v1 integration point for framework-neutral header setting.
func ApplyBearerChallengeToParser(setHeader func(name, value string), c BearerChallenge) {
	setHeader("WWW-Authenticate", FormatBearerChallenge(c))
}

// escapeQuotedString escapes a string for safe inclusion in an RFC 7235
// quoted-string value. It escapes backslash and double-quote characters
// and rejects control characters (except tab) that are not allowed in
// quoted-string content. Characters that cannot be safely escaped are
// replaced with a space to prevent header injection.
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
			// Control characters (except tab) are not allowed in
			// quoted-string. Replace with space to prevent injection.
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidateErrorURI validates that the error_uri is a valid URI reference
// (absolute or relative). This prevents injection of malformed URIs into
// the WWW-Authenticate header.
func ValidateErrorURI(uri string) error {
	if uri == "" {
		return nil
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("httpsemantics: invalid error_uri: %w", err)
	}
	// Reject URIs with control characters or whitespace that could
	// break the header format.
	if strings.ContainsAny(parsed.String(), "\r\n") {
		return fmt.Errorf("httpsemantics: error_uri contains CRLF")
	}
	return nil
}
