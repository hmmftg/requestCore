package httpsemantics

import (
	"net/http"
	"testing"
)

func TestApplyTokenResponseNoStore(t *testing.T) {
	h := http.Header{}
	ApplyTokenResponseNoStore(h)
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
	if got := h.Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want %q", got, "no-cache")
	}
}

func TestApplyTokenResponseNoStoreToParser(t *testing.T) {
	var headers map[string]string
	setHeader := func(name, value string) {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers[name] = value
	}
	ApplyTokenResponseNoStoreToParser(setHeader)
	if headers["Cache-Control"] != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", headers["Cache-Control"], "no-store")
	}
	if headers["Pragma"] != "no-cache" {
		t.Errorf("Pragma = %q, want %q", headers["Pragma"], "no-cache")
	}
}

func TestFormatBearerChallenge_DefaultRealm(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{})
	if got != `Bearer realm="Protected"` {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, `Bearer realm="Protected"`)
	}
}

func TestFormatBearerChallenge_CustomRealm(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{Realm: "My API"})
	if got != `Bearer realm="My API"` {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, `Bearer realm="My API"`)
	}
}

func TestFormatBearerChallenge_InvalidToken(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{
		Error:            BearerErrorInvalidToken,
		ErrorDescription: "The access token expired",
		ErrorURI:         "https://example.com/oauth/errors",
	})
	want := `Bearer realm="Protected", error="invalid_token", error_description="The access token expired", error_uri="https://example.com/oauth/errors"`
	if got != want {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, want)
	}
}

func TestFormatBearerChallenge_InsufficientScope(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{
		Error: BearerErrorInsufficientScope,
		Scope: "read write admin",
	})
	want := `Bearer realm="Protected", error="insufficient_scope", scope="read write admin"`
	if got != want {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, want)
	}
}

func TestFormatBearerChallenge_MissingToken(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{
		Error: BearerErrorMissingToken,
	})
	want := `Bearer realm="Protected", error="missing_token"`
	if got != want {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, want)
	}
}

func TestFormatBearerChallenge_NoError(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{
		Realm: "Protected Resource",
	})
	want := `Bearer realm="Protected Resource"`
	if got != want {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, want)
	}
}

func TestApplyBearerChallenge(t *testing.T) {
	h := http.Header{}
	ApplyBearerChallenge(h, BearerChallenge{Error: BearerErrorInvalidToken})
	if got := h.Get("WWW-Authenticate"); got == "" {
		t.Error("WWW-Authenticate not set")
	}
	if got := h.Get("WWW-Authenticate"); got != `Bearer realm="Protected", error="invalid_token"` {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

func TestApplyBearerChallengeToParser(t *testing.T) {
	var headers map[string]string
	setHeader := func(name, value string) {
		if headers == nil {
			headers = make(map[string]string)
		}
		headers[name] = value
	}
	ApplyBearerChallengeToParser(setHeader, BearerChallenge{Error: BearerErrorInvalidToken})
	if headers["WWW-Authenticate"] == "" {
		t.Error("WWW-Authenticate not set")
	}
}

func TestEscapeQuotedString_EscapesBackslash(t *testing.T) {
	got := escapeQuotedString(`a\b`)
	if got != `a\\b` {
		t.Errorf("escapeQuotedString() = %q, want %q", got, `a\\b`)
	}
}

func TestEscapeQuotedString_EscapesQuote(t *testing.T) {
	got := escapeQuotedString(`a"b`)
	if got != `a\"b` {
		t.Errorf("escapeQuotedString() = %q, want %q", got, `a\"b`)
	}
}

func TestEscapeQuotedString_ReplacesControlChars(t *testing.T) {
	got := escapeQuotedString("a\x00b\x01c")
	if got != "a b c" {
		t.Errorf("escapeQuotedString() = %q, want %q", got, "a b c")
	}
}

func TestEscapeQuotedString_PreservesTab(t *testing.T) {
	got := escapeQuotedString("a\tb")
	if got != "a\tb" {
		t.Errorf("escapeQuotedString() = %q, want %q", got, "a\tb")
	}
}

func TestEscapeQuotedString_PreventsHeaderInjection(t *testing.T) {
	// Attempt to inject a new header via CRLF. Both \r and \n are
	// control characters (< 0x20) and are replaced with spaces.
	got := escapeQuotedString("evil\r\nX-Injected: yes")
	if got != "evil  X-Injected: yes" {
		t.Errorf("escapeQuotedString() = %q, want %q", got, "evil  X-Injected: yes")
	}
}

func TestValidateErrorURI_Valid(t *testing.T) {
	if err := ValidateErrorURI("https://example.com/errors"); err != nil {
		t.Errorf("ValidateErrorURI() error = %v", err)
	}
}

func TestValidateErrorURI_Empty(t *testing.T) {
	if err := ValidateErrorURI(""); err != nil {
		t.Errorf("ValidateErrorURI() error = %v", err)
	}
}

func TestValidateErrorURI_Relative(t *testing.T) {
	if err := ValidateErrorURI("/errors/123"); err != nil {
		t.Errorf("ValidateErrorURI() error = %v", err)
	}
}
