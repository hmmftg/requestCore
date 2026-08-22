package webFramework

import (
	"net/http"

	legacy "github.com/hmmftg/requestCore/webFramework"
)

// FakeParserV2 is a test implementation of the v2 RequestParser interface.
// It embeds the v1 FakeParser and adds raw response, cookie, and
// session/flash support for testing v2 handlers and middleware.
type FakeParserV2 struct {
	legacy.FakeParser

	// ResponseStatus captures the last status code passed to SendResponse.
	ResponseStatus int

	// ResponseContentType captures the last content type passed to SendResponse.
	ResponseContentType string

	// ResponseBody captures the last body bytes passed to SendResponse.
	ResponseBody []byte

	// ResponseWritten reports whether SendResponse was called.
	ResponseWritten bool

	// Cookies stores request cookies by name.
	Cookies map[string]string

	// SetCookies captures cookies set via SetCookie.
	SetCookies []*http.Cookie

	// commitState tracks whether the response has been written.
	commitState *CommitState
}

// NewFakeParserV2 creates a FakeParserV2 with initialized maps.
func NewFakeParserV2() *FakeParserV2 {
	return &FakeParserV2{
		FakeParser: legacy.FakeParser{
			Locals:     make(map[string]any),
			ReqHeader:  make(map[string]string),
			RespHeader: make(map[string]string),
			Args:       make(map[string]string),
			Urlparams:  make(map[string]string),
		},
		Cookies: make(map[string]string),
	}
}

// SendResponse captures the response parameters for test assertions.
// If a CommitState is bound and already committed, returns nil without writing.
func (f *FakeParserV2) SendResponse(status int, contentType string, body []byte) error {
	if f.commitState != nil && f.commitState.Committed() {
		return nil
	}
	f.ResponseStatus = status
	f.ResponseContentType = contentType
	f.ResponseBody = body
	f.ResponseWritten = true
	if f.commitState != nil {
		f.commitState.MarkCommitted(status)
	}
	return nil
}

// Committed reports whether SendResponse has been called.
func (f *FakeParserV2) Committed() bool {
	if f.commitState != nil {
		return f.commitState.Committed()
	}
	return f.ResponseWritten
}

// SetCommitState binds the request's CommitState to this parser so that
// SendResponse can check and update the committed status.
func (f *FakeParserV2) SetCommitState(cs *CommitState) {
	f.commitState = cs
}

// GetCookie returns the value of the named request cookie.
func (f *FakeParserV2) GetCookie(name string) string {
	return f.Cookies[name]
}

// SetCookie captures the cookie for test assertions.
func (f *FakeParserV2) SetCookie(cookie *http.Cookie) {
	f.SetCookies = append(f.SetCookies, cookie)
}

// Reset clears all captured response state for reuse between test cases.
func (f *FakeParserV2) Reset() {
	f.ResponseStatus = 0
	f.ResponseContentType = ""
	f.ResponseBody = nil
	f.ResponseWritten = false
	f.SetCookies = nil
}
