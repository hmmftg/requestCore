// Package nextadapter bridges the new internal endpoint engine into the
// existing v2-alpha net/http and chi routers. It is fully internal and
// is the only v2 package permitted to import the root webFramework
// package, solely to forward telemetry into the mandatory AddLog
// pipeline.
//
// The bridge converts a v2-alpha *webFramework.RequestContext into a
// new-kernel *request.Context, runs the internal endpoint executor
// against a parser-backed transport, and writes the mandatory
// <operation>-req / <operation>-req-failed AddLog entries.
package nextadapter

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/libNetHttp"
	"github.com/hmmftg/requestCore/v2/request"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// maxBodyReadDefault is the default body read limit when the endpoint
// binding plan does not specify MaxBodyBytes. It prevents unbounded
// buffering for body-binding requests that did not configure a limit.
const maxBodyReadDefault int64 = 1 << 20 // 1 MiB

// buildContext constructs a new-kernel *request.Context from a v2-alpha
// *webFramework.RequestContext. It extracts the native *http.Request,
// clones headers, query, cookies, and remote address, resolves path
// parameters from the parser, and reads the body for body-binding modes.
//
// The alpha before-commit hook runner is bridged into the new context
// exactly once via AddBeforeCommitHook so the internal executor runs it
// before durability; the parser's later hook-runner call is an
// idempotent no-op.
//
// opPattern is the canonical operation pattern (e.g. "/users/{id}")
// used to resolve path parameter names. plan is the endpoint binding
// plan, used to decide whether to read the body and to enforce
// MaxBodyBytes.
func buildContext(rc *v2wf.RequestContext, opPattern string, plan binding.Plan) (*request.Context, error) {
	if rc == nil {
		return nil, errors.New("nextadapter: nil RequestContext")
	}
	if rc.Parser == nil {
		return nil, errors.New("nextadapter: nil Parser")
	}
	if rc.LegacyContext == nil {
		return nil, errors.New("nextadapter: nil LegacyContext")
	}

	httpReq := libNetHttp.GetHTTPRequest(rc.LegacyContext)
	if httpReq == nil {
		return nil, errors.New("nextadapter: could not extract *http.Request from LegacyContext")
	}

	opts := []request.Option{
		request.WithMethod(rc.Parser.GetMethod()),
		request.WithPath(rc.Parser.GetPath()),
		request.WithRoutePattern(opPattern),
		request.WithHeader(cloneHeader(httpReq.Header)),
		request.WithQuery(httpReq.URL.Query()),
		request.WithCookies(httpReq.Cookies()),
		request.WithRemoteAddr(httpReq.RemoteAddr),
		request.WithNative(httpReq),
	}

	// Resolve path parameters from the canonical operation pattern.
	if pathParams := resolvePathParams(opPattern, rc.Parser.GetURLParam); len(pathParams) > 0 {
		opts = append(opts, request.WithPathParams(pathParams))
	}

	// Read the body only for body-binding modes.
	if plan.Mode == binding.ModeJSON || plan.Mode == binding.ModeForm {
		body, err := readBoundedBody(httpReq, plan.MaxBodyBytes)
		if err != nil {
			return nil, err
		}
		opts = append(opts, request.WithBody(body))
	}

	reqCtx := request.NewContext(rc.Context, opts...)

	// Bridge the alpha before-commit hook runner into the new context.
	// The internal executor runs it before durability; the parser's
	// later hook-runner call is an idempotent no-op because
	// RunBeforeCommitHooks on the alpha context records hooksRan.
	reqCtx.AddBeforeCommitHook(rc.RunBeforeCommitHooks)

	return reqCtx, nil
}

// cloneHeader returns a deep copy of the given header map.
func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return make(http.Header)
	}
	cp := make(http.Header, len(h))
	for k, vs := range h {
		cp[k] = append([]string(nil), vs...)
	}
	return cp
}

// resolvePathParams extracts path parameter names from the canonical
// operation pattern (e.g. "/users/{id}") and resolves each through the
// provided getter. This works for both chi-populated parser params and
// Go ServeMux Request.PathValue fallback without importing chi.
func resolvePathParams(pattern string, getter func(name string) string) map[string]string {
	params := parsePatternParams(pattern)
	if len(params) == 0 {
		return nil
	}
	result := make(map[string]string, len(params))
	for _, name := range params {
		if v := getter(name); v != "" {
			result[name] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parsePatternParams extracts the parameter names from a canonical
// pattern using {name} syntax.
func parsePatternParams(pattern string) []string {
	var names []string
	for {
		start := strings.Index(pattern, "{")
		if start == -1 {
			break
		}
		end := strings.Index(pattern[start:], "}")
		if end == -1 {
			break
		}
		end += start
		name := pattern[start+1 : end]
		// Strip multi-segment wildcard suffix (e.g. "path...").
		name = strings.TrimSuffix(name, "...")
		if name != "" {
			names = append(names, name)
		}
		pattern = pattern[end+1:]
	}
	return names
}

// readBoundedBody reads at most limit+1 bytes from the request body so
// the binding layer can produce its normal 413 error without
// pre-buffering an unbounded body. If limit is 0, maxBodyReadDefault is
// used. The consumed prefix is restored in front of the unread original
// request body through a replaying io.ReadCloser so downstream code can
// still see the complete stream.
func readBoundedBody(req *http.Request, limit int64) (string, error) {
	if limit <= 0 {
		limit = maxBodyReadDefault
	}
	if req.Body == nil {
		return "", nil
	}
	limited := io.LimitReader(req.Body, limit+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("nextadapter: read body: %w", err)
	}
	// Restore the consumed prefix in front of the unread original body.
	req.Body = replayingBody{prefix: buf, source: req.Body}
	return string(buf), nil
}

// replayingBody yields a buffered prefix followed by the remaining
// unread bytes of the original request body. Close is delegated to the
// original body.
type replayingBody struct {
	prefix []byte
	source io.ReadCloser
}

func (r replayingBody) Read(p []byte) (int, error) {
	if len(r.prefix) > 0 {
		n := copy(p, r.prefix)
		r.prefix = r.prefix[n:]
		return n, nil
	}
	if r.source == nil {
		return 0, io.EOF
	}
	return r.source.Read(p)
}

func (r replayingBody) Close() error {
	if r.source == nil {
		return nil
	}
	return r.source.Close()
}
