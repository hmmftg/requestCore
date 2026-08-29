package request

import (
	"context"
	"net/http"
	"net/url"
	"sync"
)

// Context holds the per-request state passed through the redesigned v2
// handler and middleware pipeline. It is stdlib-only and framework-neutral.
//
// Handlers receive *Context and their typed request input, mutate
// response metadata via ctx.Response(), and return their typed response.
// The Context provides access to request data, response state, identity,
// typed key-value storage, before-commit hooks, and request-side native
// data.
//
// Framework-native response writing is NOT available through Context.
// The Native() method returns request-side native data only.
type Context struct {
	// ctx is the Go context for cancellation and tracing.
	ctx context.Context

	// Request metadata
	method      string
	path        string
	routePattern string
	header      http.Header
	query       url.Values
	pathParams  map[string]string
	cookies     []*http.Cookie
	remoteAddr  string

	// Response state
	response *ResponseState

	// Identity and tracing
	principal any
	requestID string
	traceID   string

	// Native request-side data (e.g. *http.Request for net/http/chi)
	native any

	// Typed key-value store
	valuesMu sync.RWMutex
	values   map[uint64]any

	// Before-commit hooks
	hooksMu  sync.Mutex
	hooks    []func() error
	hooksRan bool
}

// Option configures a Context at construction time. Adapters and the
// fake transport use options to populate request data.
type Option func(*Context)

// NewContext creates a new Context with the given Go context and options.
// The response state is initialized with defaults (status 200, empty headers).
func NewContext(goCtx context.Context, opts ...Option) *Context {
	c := &Context{
		ctx:      goCtx,
		response: NewResponseState(),
		values:   make(map[uint64]any),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithMethod sets the HTTP method.
func WithMethod(method string) Option {
	return func(c *Context) { c.method = method }
}

// WithPath sets the request path.
func WithPath(path string) Option {
	return func(c *Context) { c.path = path }
}

// WithRoutePattern sets the registered route pattern (e.g. "/users/{id}").
func WithRoutePattern(pattern string) Option {
	return func(c *Context) { c.routePattern = pattern }
}

// WithHeader sets the request header map.
func WithHeader(h http.Header) Option {
	return func(c *Context) { c.header = h }
}

// WithQuery sets the parsed query parameters.
func WithQuery(q url.Values) Option {
	return func(c *Context) { c.query = q }
}

// WithPathParams sets the path parameters map.
func WithPathParams(params map[string]string) Option {
	return func(c *Context) { c.pathParams = params }
}

// WithCookies sets the request cookies.
func WithCookies(cookies []*http.Cookie) Option {
	return func(c *Context) { c.cookies = cookies }
}

// WithRemoteAddr sets the remote address.
func WithRemoteAddr(addr string) Option {
	return func(c *Context) { c.remoteAddr = addr }
}

// WithNative sets the request-side native data.
func WithNative(native any) Option {
	return func(c *Context) { c.native = native }
}

// Context returns the underlying Go context for cancellation and tracing.
func (c *Context) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// WithContext returns a new Context with the given Go context, copying
// all other state. This is used by middleware that needs to derive a
// child context (e.g. for deadlines).
func (c *Context) WithContext(goCtx context.Context) *Context {
	nc := &Context{
		ctx:          goCtx,
		method:       c.method,
		path:         c.path,
		routePattern: c.routePattern,
		header:       c.header,
		query:        c.query,
		pathParams:   c.pathParams,
		cookies:      c.cookies,
		remoteAddr:   c.remoteAddr,
		response:     c.response,
		principal:    c.principal,
		requestID:    c.requestID,
		traceID:      c.traceID,
		native:       c.native,
		values:       c.values,
	}
	return nc
}

// Method returns the HTTP method.
func (c *Context) Method() string { return c.method }

// Path returns the request path.
func (c *Context) Path() string { return c.path }

// RoutePattern returns the registered route pattern, or "" if not set.
func (c *Context) RoutePattern() string { return c.routePattern }

// Header returns the value of the named request header, or "" if not present.
func (c *Context) Header(name string) string {
	if c.header == nil {
		return ""
	}
	return c.header.Get(name)
}

// Headers returns a copy of the request header map.
func (c *Context) Headers() http.Header {
	if c.header == nil {
		return make(http.Header)
	}
	h := make(http.Header, len(c.header))
	for k, v := range c.header {
		h[k] = append([]string(nil), v...)
	}
	return h
}

// Query returns the first value for the named query parameter, or "" if
// not present.
func (c *Context) Query(name string) string {
	if c.query == nil {
		return ""
	}
	return c.query.Get(name)
}

// QueryAll returns all values for the named query parameter.
func (c *Context) QueryAll(name string) []string {
	if c.query == nil {
		return nil
	}
	return c.query[name]
}

// PathParam returns the value of the named path parameter, or "" if not
// present.
func (c *Context) PathParam(name string) string {
	if c.pathParams == nil {
		return ""
	}
	return c.pathParams[name]
}

// PathParams returns a copy of all path parameters.
func (c *Context) PathParams() map[string]string {
	if c.pathParams == nil {
		return make(map[string]string)
	}
	m := make(map[string]string, len(c.pathParams))
	for k, v := range c.pathParams {
		m[k] = v
	}
	return m
}

// Cookie returns the named request cookie, or nil if not present.
func (c *Context) Cookie(name string) *http.Cookie {
	for _, c2 := range c.cookies {
		if c2.Name == name {
			return c2
		}
	}
	return nil
}

// Cookies returns all request cookies.
func (c *Context) Cookies() []*http.Cookie {
	return c.cookies
}

// RemoteAddr returns the remote address of the client.
func (c *Context) RemoteAddr() string { return c.remoteAddr }

// Response returns the canonical response metadata. Handlers mutate
// status and headers through this object before returning their
// response value.
func (c *Context) Response() *ResponseState { return c.response }

// Principal returns the authenticated principal, or nil if not set.
// The return type is any to avoid coupling request to a specific
// principal type; middleware packages define the concrete type and
// store it via SetPrincipal.
func (c *Context) Principal() any { return c.principal }

// RequestID returns the request identifier, or "" if not set.
func (c *Context) RequestID() string { return c.requestID }

// TraceID returns the trace identifier, or "" if not set.
func (c *Context) TraceID() string { return c.traceID }

// Native returns request-side native data. The concrete type depends on
// the adapter:
//   - net/http and chi: *http.Request
//   - Gin: the incoming *http.Request (not *gin.Context)
//   - Fiber: request-side fasthttp data or a read-only adapter view
//
// Framework-native response writing is NOT available through Native.
// Files, streams, redirects, and no-content responses go through
// canonical response helpers so hooks and commit state remain correct.
func (c *Context) Native() any { return c.native }

// SetPrincipal sets the authenticated principal. Called by auth middleware.
func (c *Context) SetPrincipal(p any) { c.principal = p }

// SetRequestID sets the request identifier. Called by request-ID middleware.
func (c *Context) SetRequestID(id string) { c.requestID = id }

// SetTraceID sets the trace identifier. Called by tracing middleware.
func (c *Context) SetTraceID(id string) { c.traceID = id }

// setTyped stores a value under a typed key ID.
func (c *Context) setTyped(id uint64, value any) {
	c.valuesMu.Lock()
	defer c.valuesMu.Unlock()
	if c.values == nil {
		c.values = make(map[uint64]any)
	}
	c.values[id] = value
}

// getTyped retrieves a value by typed key ID.
func (c *Context) getTyped(id uint64) (any, bool) {
	c.valuesMu.RLock()
	defer c.valuesMu.RUnlock()
	v, ok := c.values[id]
	return v, ok
}

// AddBeforeCommitHook registers a function to be invoked before the
// response is committed. Hooks fire in registration order. A hook
// returning a non-nil error aborts the commit (strict semantics).
func (c *Context) AddBeforeCommitHook(hook func() error) {
	if hook == nil {
		return
	}
	c.hooksMu.Lock()
	defer c.hooksMu.Unlock()
	c.hooks = append(c.hooks, hook)
}

// RunBeforeCommitHooks invokes all registered before-commit hooks in
// order. This method is idempotent: the first call runs all hooks and
// subsequent calls return nil without re-running them. All hooks are
// run even if one fails; the first error is returned.
func (c *Context) RunBeforeCommitHooks() error {
	c.hooksMu.Lock()
	if c.hooksRan {
		c.hooksMu.Unlock()
		return nil
	}
	c.hooksRan = true
	hooks := append([]func() error(nil), c.hooks...)
	c.hooksMu.Unlock()
	var firstErr error
	for _, h := range hooks {
		if err := h(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
