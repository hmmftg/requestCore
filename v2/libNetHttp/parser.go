// Package libNetHttp provides the v2 net/http web framework adapter for requestCore.
//
// It embeds the v1 [github.com/hmmftg/requestCore/libNetHttp.NetHTTPParser]
// and adds raw response writing, cookie access, and v2 routing support.
package libNetHttp

import (
	"net/http"

	legacyLibNetHttp "github.com/hmmftg/requestCore/libNetHttp"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// NetHTTPParserV2 extends the v1 NetHTTPParser with raw response writing
// and cookie access for v2 renderer and session support.
type NetHTTPParserV2 struct {
	*legacyLibNetHttp.NetHTTPParser

	// commitState tracks whether the response has been written.
	// SendResponse checks and updates this to avoid double-writes.
	commitState *v2wf.CommitState
}

// InitContextV2 creates a NetHTTPParserV2 from an HTTP request and response writer.
func InitContextV2(r *http.Request, w http.ResponseWriter) *NetHTTPParserV2 {
	return &NetHTTPParserV2{
		NetHTTPParser: legacyLibNetHttp.InitContext(r, w),
	}
}

// SendResponse writes a raw response with the given status, content type,
// and body bytes to the HTTP response writer. If a CommitState is bound and
// already committed, returns nil without writing.
func (p *NetHTTPParserV2) SendResponse(status int, contentType string, body []byte) error {
	if p.commitState != nil && p.commitState.Committed() {
		return nil
	}
	if contentType != "" {
		p.Response.Header().Set("Content-Type", contentType)
	}
	p.Response.WriteHeader(status)
	var err error
	if len(body) > 0 {
		_, err = p.Response.Write(body)
	}
	if err == nil && p.commitState != nil {
		p.commitState.MarkCommitted(status)
	}
	return err
}

// GetCookie returns the value of the named request cookie.
func (p *NetHTTPParserV2) GetCookie(name string) string {
	cookie, err := p.Request.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetURLParam returns a URL path parameter by name. It first checks the
// parser's Params map (populated by chi or manual setup), then falls back
// to Go 1.22+ ServeMux's req.PathValue().
func (p *NetHTTPParserV2) GetURLParam(name string) string {
	if v, ok := p.Params[name]; ok {
		return v
	}
	return p.Request.PathValue(name)
}

// CheckURLParam returns a URL path parameter and whether it exists.
func (p *NetHTTPParserV2) CheckURLParam(name string) (string, bool) {
	if v, ok := p.Params[name]; ok {
		return v, true
	}
	v := p.Request.PathValue(name)
	return v, v != ""
}

// SetCookie sets an HTTP response cookie.
func (p *NetHTTPParserV2) SetCookie(cookie *http.Cookie) {
	http.SetCookie(p.Response, cookie)
}

// SetCommitState binds the request's CommitState to this parser so that
// SendResponse can check and update the committed status.
func (p *NetHTTPParserV2) SetCommitState(cs *v2wf.CommitState) {
	p.commitState = cs
}
