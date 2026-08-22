// Package libGin provides the v2 Gin web framework adapter for requestCore.
//
// It embeds the v1 [github.com/hmmftg/requestCore/libGin.GinParser] and
// adds raw response writing, cookie access, and v2 routing support.
package libGin

import (
	"net/http"

	legacyLibGin "github.com/hmmftg/requestCore/libGin"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// GinParserV2 extends the v1 GinParser with raw response writing and
// cookie access for v2 renderer and session support.
type GinParserV2 struct {
	legacyLibGin.GinParser

	// commitState tracks whether the response has been written.
	// SendResponse checks and updates this to avoid double-writes.
	commitState *v2wf.CommitState

	// hookRunner runs before-commit hooks before the response is written.
	hookRunner func() error
}

// InitContextV2 creates a GinParserV2 from a Gin context.
func InitContextV2(c any) *GinParserV2 {
	return &GinParserV2{
		GinParser: legacyLibGin.InitContext(c),
	}
}

// SendResponse writes a raw response with the given status, content type,
// and body bytes to the Gin response writer. If a CommitState is bound and
// already committed, this method returns nil without writing. Before writing,
// the before-commit hook runner is invoked (if set) so that session cookies
// and other pre-write side effects are persisted even for direct parser writes.
func (p *GinParserV2) SendResponse(status int, contentType string, body []byte) error {
	if p.commitState != nil && p.commitState.Committed() {
		return nil
	}
	if p.hookRunner != nil {
		if err := p.hookRunner(); err != nil {
			return err
		}
	}
	if contentType != "" {
		p.Ctx.Header("Content-Type", contentType)
	}
	p.Ctx.Data(status, contentType, body)
	if p.commitState != nil {
		p.commitState.MarkCommitted(status)
	}
	return nil
}

// GetCookie returns the value of the named request cookie.
func (p *GinParserV2) GetCookie(name string) string {
	cookie, err := p.Ctx.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie
}

// SetCookie sets an HTTP response cookie on the Gin response.
func (p *GinParserV2) SetCookie(cookie *http.Cookie) {
	http.SetCookie(p.Ctx.Writer, cookie)
}

// SetCommitState binds the request's CommitState to this parser so that
// SendResponse can check and update the committed status.
func (p *GinParserV2) SetCommitState(cs *v2wf.CommitState) {
	p.commitState = cs
}

// SetBeforeCommitHookRunner binds a function that runs before-commit hooks
// before SendResponse writes the response.
func (p *GinParserV2) SetBeforeCommitHookRunner(fn func() error) {
	p.hookRunner = fn
}
