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
)

// FiberParserV2 extends the v1 FiberParser with raw response writing and
// cookie access for v2 renderer and session support.
type FiberParserV2 struct {
	legacyLibFiber.FiberParser
}

// InitContextV2 creates a FiberParserV2 from a Fiber context.
func InitContextV2(c *fiber.Ctx) FiberParserV2 {
	return FiberParserV2{
		FiberParser: legacyLibFiber.InitContext(c),
	}
}

// SendResponse writes a raw response with the given status, content type,
// and body bytes to the Fiber response. This bypasses net/http entirely
// and writes directly to the fasthttp response.
func (p FiberParserV2) SendResponse(status int, contentType string, body []byte) error {
	if contentType != "" {
		p.Ctx.Set("Content-Type", contentType)
	}
	p.Ctx.Status(status)
	return p.Ctx.Send(body)
}

// GetCookie returns the value of the named request cookie.
func (p FiberParserV2) GetCookie(name string) string {
	return p.Ctx.Cookies(name)
}

// SetCookie sets an HTTP response cookie on the Fiber response.
// Fiber's cookie API differs from net/http, so we map the http.Cookie
// fields to Fiber's cookie methods.
func (p FiberParserV2) SetCookie(cookie *http.Cookie) {
	p.Ctx.Cookie(&fiber.Cookie{
		Name:     cookie.Name,
		Value:    cookie.Value,
		Path:     cookie.Path,
		Domain:   cookie.Domain,
		MaxAge:   cookie.MaxAge,
		Secure:   cookie.Secure,
		HTTPOnly: cookie.HttpOnly,
	})
}
