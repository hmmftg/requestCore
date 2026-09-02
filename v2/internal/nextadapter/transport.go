package nextadapter

import (
	"net/http"

	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// parserTransport implements the internal endpoint.Transport interface
// by delegating writes to the existing v2-alpha parser's SendResponse.
// It applies all final response headers through SetRespHeader before
// calling SendResponse. The parser's own before-commit hook runner is
// an idempotent safety net; the canonical hook execution happens inside
// the internal executor via the bridged request.Context hook.
type parserTransport struct {
	rc *v2wf.RequestContext
}

// WriteResponse applies final headers and writes the response through
// the alpha parser. If the alpha context is already committed, no
// write is attempted.
func (t *parserTransport) WriteResponse(status int, contentType string, headers http.Header, body []byte) error {
	if t.rc == nil || t.rc.Committed() {
		return nil
	}
	// Apply all final response headers before writing.
	for k, vs := range headers {
		for _, v := range vs {
			t.rc.Parser.SetRespHeader(k, v)
		}
	}
	return t.rc.Parser.SendResponse(status, contentType, body)
}

// Committed reports whether the alpha response has been committed.
func (t *parserTransport) Committed() bool {
	if t.rc == nil {
		return false
	}
	return t.rc.Committed()
}
