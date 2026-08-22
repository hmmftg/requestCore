// Package libGin provides the v2 Gin web framework adapter for requestCore.
//
// It embeds the v1 [github.com/hmmftg/requestCore/libGin.GinParser] and
// adds raw response writing, cookie access, and v2 routing support.
package libGin

import (
	"net/http"

	legacyLibGin "github.com/hmmftg/requestCore/libGin"
)

// GinParserV2 extends the v1 GinParser with raw response writing and
// cookie access for v2 renderer and session support.
type GinParserV2 struct {
	legacyLibGin.GinParser
}

// InitContextV2 creates a GinParserV2 from a Gin context.
func InitContextV2(c any) GinParserV2 {
	return GinParserV2{
		GinParser: legacyLibGin.InitContext(c),
	}
}

// SendResponse writes a raw response with the given status, content type,
// and body bytes to the Gin response writer.
func (p GinParserV2) SendResponse(status int, contentType string, body []byte) error {
	if contentType != "" {
		p.Ctx.Header("Content-Type", contentType)
	}
	p.Ctx.Data(status, contentType, body)
	return nil
}

// GetCookie returns the value of the named request cookie.
func (p GinParserV2) GetCookie(name string) string {
	cookie, err := p.Ctx.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie
}

// SetCookie sets an HTTP response cookie on the Gin response.
func (p GinParserV2) SetCookie(cookie *http.Cookie) {
	http.SetCookie(p.Ctx.Writer, cookie)
}
