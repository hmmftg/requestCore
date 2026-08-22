// Package libFiber provides the v2 Fiber web framework adapter for requestCore.
//
// It embeds the v1 [github.com/hmmftg/requestCore/libFiber.FiberParser] and
// adds raw response writing, cookie access, and v2 routing support.
//
// Fiber uses fasthttp (not net/http), so the SendResponse method writes
// directly to the fiber.Ctx response rather than through http.ResponseWriter.
package libFiber

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	legacyLibFiber "github.com/hmmftg/requestCore/libFiber"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// FiberParserV2 extends the v1 FiberParser with raw response writing and
// cookie access for v2 renderer and session support.
type FiberParserV2 struct {
	legacyLibFiber.FiberParser

	// commitState tracks whether the response has been written.
	// SendResponse checks and updates this to avoid double-writes.
	commitState *v2wf.CommitState
}

// InitContextV2 creates a FiberParserV2 from a Fiber context.
func InitContextV2(c *fiber.Ctx) *FiberParserV2 {
	return &FiberParserV2{
		FiberParser: legacyLibFiber.InitContext(c),
	}
}

// SendResponse writes a raw response with the given status, content type,
// and body bytes to the Fiber response. This bypasses net/http entirely
// and writes directly to the fasthttp response.
// If a CommitState is bound and already committed, returns nil without writing.
func (p *FiberParserV2) SendResponse(status int, contentType string, body []byte) error {
	if p.commitState != nil && p.commitState.Committed() {
		return nil
	}
	if contentType != "" {
		p.Ctx.Set("Content-Type", contentType)
	}
	p.Ctx.Status(status)
	err := p.Ctx.Send(body)
	if err == nil && p.commitState != nil {
		p.commitState.MarkCommitted(status)
	}
	return err
}

// GetCookie returns the value of the named request cookie.
func (p *FiberParserV2) GetCookie(name string) string {
	return p.Ctx.Cookies(name)
}

// SetCookie sets an HTTP response cookie on the Fiber response.
// Fiber's cookie API differs from net/http, so we map all http.Cookie
// fields to Fiber's cookie methods, including SameSite.
func (p *FiberParserV2) SetCookie(cookie *http.Cookie) {
	fc := &fiber.Cookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Path:     cookie.Path,
		Domain:   cookie.Domain,
		MaxAge:   cookie.MaxAge,
		Secure:   cookie.Secure,
		HTTPOnly: cookie.HttpOnly,
	}
	// Map SameSite.
	switch cookie.SameSite {
	case http.SameSiteStrictMode:
		fc.SameSite = "strict"
	case http.SameSiteNoneMode:
		fc.SameSite = "none"
	case http.SameSiteLaxMode:
		fc.SameSite = "lax"
	default:
		fc.SameSite = ""
	}
	// Map Expires if set.
	if !cookie.Expires.IsZero() {
		fc.Expires = cookie.Expires
	}
	p.Ctx.Cookie(fc)
}

// SetCommitState binds the request's CommitState to this parser so that
// SendResponse can check and update the committed status.
func (p *FiberParserV2) SetCommitState(cs *v2wf.CommitState) {
	p.commitState = cs
}
