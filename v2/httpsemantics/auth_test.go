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

func TestFormatBearerChallenge_DefaultRealm(t *testing.T) {
	got := FormatBearerChallenge(BearerChallenge{})
	if got != `Bearer realm="Protected"` {
		t.Errorf("FormatBearerChallenge() = %q, want %q", got, `Bearer realm="Protected"`)
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

func TestApplyBearerChallenge(t *testing.T) {
	h := http.Header{}
	ApplyBearerChallenge(h, BearerChallenge{Error: BearerErrorInvalidToken})
	if got := h.Get("WWW-Authenticate"); got == "" {
		t.Error("WWW-Authenticate not set")
	}
}

func TestEscapeQuotedString_PreventsHeaderInjection(t *testing.T) {
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
