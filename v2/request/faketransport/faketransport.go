// Package faketransport provides an internal fake HTTP transport for
// testing the redesigned v2 kernel without any adapter.
//
// FakeTransport builds a *request.Context from a synthetic *http.Request
// and captures response writes. It is internal to v2/request and cannot
// be imported by external packages. Tranche 8 will create a public
// testkit package that may wrap this transport.
package faketransport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/hmmftg/requestCore/v2/request"
)

// FakeTransport is an internal fake HTTP transport for kernel testing.
// It builds a request.Context and captures response writes without
// requiring any adapter.
type FakeTransport struct {
	ctx       *request.Context
	recorder  *httptest.ResponseRecorder
	committed bool
}

// config holds the accumulated request configuration before the
// context is built.
type config struct {
	method       string
	path         string
	routePattern string
	header       http.Header
	query        url.Values
	pathParams   map[string]string
	cookies      []*http.Cookie
	remoteAddr   string
	native       any
	body         string
}

// Option configures the FakeTransport at construction time.
type Option func(*config)

// New creates a FakeTransport for a request with the given method and
// path. Options can add body, headers, query, path params, cookies,
// and remote address.
func New(method, path string, opts ...Option) *FakeTransport {
	cfg := &config{
		method:     method,
		path:       path,
		header:     make(http.Header),
		query:      make(url.Values),
		pathParams: make(map[string]string),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Parse query from path if present.
	pathOnly := cfg.path
	if idx := strings.Index(cfg.path, "?"); idx >= 0 {
		pathOnly = cfg.path[:idx]
		parsedQuery, _ := url.ParseQuery(cfg.path[idx+1:])
		for k, v := range parsedQuery {
			if _, ok := cfg.query[k]; !ok {
				cfg.query[k] = v
			}
		}
	}

	reqOpts := []request.Option{
		request.WithMethod(cfg.method),
		request.WithPath(pathOnly),
		request.WithHeader(cfg.header),
		request.WithQuery(cfg.query),
		request.WithPathParams(cfg.pathParams),
		request.WithCookies(cfg.cookies),
	}
	if cfg.routePattern != "" {
		reqOpts = append(reqOpts, request.WithRoutePattern(cfg.routePattern))
	}
	if cfg.remoteAddr != "" {
		reqOpts = append(reqOpts, request.WithRemoteAddr(cfg.remoteAddr))
	}
	if cfg.native != nil {
		reqOpts = append(reqOpts, request.WithNative(cfg.native))
	}
	if cfg.body != "" {
		reqOpts = append(reqOpts, request.WithBody(cfg.body))
	}

	goCtx := context.Background()

	return &FakeTransport{
		ctx:      request.NewContext(goCtx, reqOpts...),
		recorder: httptest.NewRecorder(),
	}
}

// WithBody sets the request body as a string.
func WithBody(body string) Option {
	return func(c *config) { c.body = body }
}

// WithHeader sets a request header.
func WithHeader(key, value string) Option {
	return func(c *config) { c.header.Set(key, value) }
}

// WithQueryParam sets a query parameter, replacing any existing value
// for the same key.
func WithQueryParam(key, value string) Option {
	return func(c *config) { c.query.Set(key, value) }
}

// WithQueryParamAdd appends a value to a query parameter, preserving
// existing values for the same key. Use this for multi-valued query
// parameters.
func WithQueryParamAdd(key, value string) Option {
	return func(c *config) { c.query.Add(key, value) }
}

// WithPathParam sets a path parameter.
func WithPathParam(key, value string) Option {
	return func(c *config) { c.pathParams[key] = value }
}

// WithRoutePattern sets the route pattern.
func WithRoutePattern(pattern string) Option {
	return func(c *config) { c.routePattern = pattern }
}

// WithCookie adds a request cookie.
func WithCookie(name, value string) Option {
	return func(c *config) {
		c.cookies = append(c.cookies, &http.Cookie{Name: name, Value: value})
	}
}

// WithRemoteAddr sets the remote address.
func WithRemoteAddr(addr string) Option {
	return func(c *config) { c.remoteAddr = addr }
}

// WithNative sets the request-side native data.
func WithNative(native any) Option {
	return func(c *config) { c.native = native }
}

// Context returns the request.Context built by the fake transport.
func (ft *FakeTransport) Context() *request.Context {
	return ft.ctx
}

// WriteResponse writes a response with the given status, content type,
// and body. This simulates the adapter transport write. The first call
// wins; subsequent calls are no-ops.
func (ft *FakeTransport) WriteResponse(status int, contentType string, body []byte) {
	if ft.committed {
		return
	}
	ft.committed = true
	if contentType != "" {
		ft.recorder.Header().Set("Content-Type", contentType)
	}
	ft.recorder.WriteHeader(status)
	_, _ = ft.recorder.Write(body)
}

// ResponseStatus returns the committed status code, or 0 if not committed.
func (ft *FakeTransport) ResponseStatus() int {
	if !ft.committed {
		return 0
	}
	return ft.recorder.Code
}

// ResponseBody returns the response body bytes, or nil if not committed.
func (ft *FakeTransport) ResponseBody() []byte {
	if !ft.committed {
		return nil
	}
	return ft.recorder.Body.Bytes()
}

// ResponseHeaders returns the response headers.
func (ft *FakeTransport) ResponseHeaders() http.Header {
	return ft.recorder.Header()
}

// Recorder returns the underlying httptest.ResponseRecorder. This is
// exposed for adapters that need direct access to the recorder (e.g.
// the internal endpoint Transport adapter). Mutating the recorder
// directly bypasses the committed flag.
func (ft *FakeTransport) Recorder() *httptest.ResponseRecorder {
	return ft.recorder
}

// MarkCommitted marks the transport as committed without writing a
// response. Used by adapters that write directly to the recorder.
func (ft *FakeTransport) MarkCommitted() {
	ft.committed = true
}

// Committed reports whether the response has been written.
func (ft *FakeTransport) Committed() bool {
	return ft.committed
}

// Body returns the request body string, or "" if none was set.
func (ft *FakeTransport) Body() string {
	return ft.ctx.Body()
}
