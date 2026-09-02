package request

import (
	"net/http"
	"sync"
)

// ResponseState holds canonical response metadata that handlers mutate
// before returning their response value. The status defaults to 200;
// headers are initialized empty. Handlers call ctx.Response().Status(x)
// and ctx.Response().Header().Set(k, v) to set dynamic status and headers.
//
// ResponseState is safe for concurrent use, though a single request is
// typically processed by one goroutine.
type ResponseState struct {
	mu        sync.RWMutex
	status    int
	statusSet bool
	headers   http.Header
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
// returns true.
func (r *ResponseState) SetStatus(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
	r.statusSet = true
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
		status:    r.status,
		statusSet: r.statusSet,
		headers:   h,
	}
}
