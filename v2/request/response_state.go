package request

import (
	"fmt"
	"net/http"
	"sync"
)

// ResponseState holds canonical response metadata that handlers mutate
// before returning their response value. The status defaults to 200;
// headers are initialized empty. Handlers call ctx.Response().Status(x)
// and ctx.Response().Header().Set(k, v) to set dynamic status and headers.
//
// ResponseState also tracks body suppression for no-content (204/205),
// not-modified (304), HEAD, and redirect responses. When body is
// suppressed, the commit engine writes status and headers only.
//
// ResponseState is safe for concurrent use, though a single request is
// typically processed by one goroutine.
type ResponseState struct {
	mu             sync.RWMutex
	status         int
	statusSet      bool
	headers        http.Header
	bodySuppressed bool
}

// NewResponseState creates a ResponseState with default status 200 and
// an empty header map. The default status is not considered explicitly
// set; StatusSet returns false until SetStatus is called.
func NewResponseState() *ResponseState {
	return &ResponseState{
		status:  200,
		headers: make(http.Header),
	}
}

// Status returns the currently set response status code.
func (r *ResponseState) Status() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// StatusSet reports whether the status was explicitly set via SetStatus.
// The default 200 status is not considered explicitly set.
func (r *ResponseState) StatusSet() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.statusSet
}

// SetStatus sets the response status code. This is used by handlers to
// set a dynamic status (e.g. 201 for Create). The status is read by the
// commit engine when preparing the response. After this call StatusSet
// returns true. The status must be a valid HTTP status code (100-599).
func (r *ResponseState) SetStatus(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !isValidHTTPStatus(status) {
		panic(fmt.Sprintf("request: invalid HTTP status code %d", status))
	}
	r.status = status
	r.statusSet = true
}

// BodySuppressed reports whether the response body should be suppressed.
// This is true for no-content, redirect, and HEAD responses.
func (r *ResponseState) BodySuppressed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bodySuppressed
}

// SuppressBody marks the response as having no body. The commit engine
// will write status and headers only. This is used by NoContent,
// Redirect, and HEAD handlers.
func (r *ResponseState) SuppressBody() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bodySuppressed = true
}

// NoContent sets the response to 204 No Content with a suppressed body.
// Any previously set headers are preserved.
func (r *ResponseState) NoContent() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = http.StatusNoContent
	r.statusSet = true
	r.bodySuppressed = true
}

// Redirect sets the response to a redirect status with a Location header
// and a suppressed body. The status must be a valid redirect code
// (301, 302, 303, 307, or 308).
func (r *ResponseState) Redirect(status int, url string) {
	if !isRedirectStatus(status) {
		panic(fmt.Sprintf("request: invalid redirect status %d", status))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
	r.statusSet = true
	r.bodySuppressed = true
	r.headers.Set("Location", url)
}

// isValidHTTPStatus reports whether the given code is a valid HTTP
// response status code (100-599).
func isValidHTTPStatus(code int) bool {
	return code >= 100 && code <= 599
}

// isRedirectStatus reports whether the given code is a valid redirect
// status code that carries a Location header.
func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	}
	return false
}

// IsNoBodyStatus reports whether the given HTTP status code should
// suppress the response body per RFC 9110. This includes 1xx, 204,
// 205, and 304.
func IsNoBodyStatus(status int) bool {
	return (status >= 100 && status < 200) ||
		status == http.StatusNoContent ||
		status == http.StatusResetContent ||
		status == http.StatusNotModified
}

// Header returns the response header map. Callers may mutate the
// returned map directly (e.g. Header().Set("Location", url)). The map
// is safe for concurrent access via the underlying sync.RWMutex, but
// callers should avoid concurrent mutation in practice.
func (r *ResponseState) Header() http.Header {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// Return a copy to prevent untracked mutation. The commit engine
	// reads this copy when preparing the response.
	h := make(http.Header, len(r.headers))
	for k, v := range r.headers {
		h[k] = append([]string(nil), v...)
	}
	return h
}

// SetHeader sets a response header value, replacing any existing value.
func (r *ResponseState) SetHeader(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.headers.Set(key, value)
}

// AddHeader adds a response header value without replacing existing values.
func (r *ResponseState) AddHeader(key, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.headers.Add(key, value)
}

// Clone returns a snapshot copy of the response state for immutable
// replay/cache storage. The clone has its own header map copy.
func (r *ResponseState) Clone() *ResponseState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h := make(http.Header, len(r.headers))
	for k, v := range r.headers {
		h[k] = append([]string(nil), v...)
	}
	return &ResponseState{
		status:         r.status,
		statusSet:      r.statusSet,
		headers:        h,
		bodySuppressed: r.bodySuppressed,
	}
}
