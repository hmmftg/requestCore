// Package request provides the stdlib-only request Context, typed value
// keys, and response metadata for the redesigned v2 kernel.
//
// The Context carries per-request state through the handler and middleware
// pipeline: method, path, route pattern, headers, query, path params,
// cookies, remote address, response metadata, principal, request ID,
// trace ID, typed key-value store, before-commit hooks, and request-side
// native data.
//
// request imports only the Go standard library. It does not import
// response, telemetry, adapters, session, app, or any v1 package.
package request

import "sync/atomic"

// Key is a comparable, type-safe identifier for storing values in a
// Context. Keys carry an unexported unique identity (a process-wide
// counter) so that two keys with the same name from different call sites
// are distinct. The name is used only for diagnostics.
//
// Create keys with NewKey[T] at package initialization time and reuse
// them; do not create keys per request.
type Key[T any] struct {
	id   uint64
	name string
}

var keyCounter atomic.Uint64

// NewKey creates a new typed Key with the given diagnostic name. Each
// call returns a distinct key even if the name is the same.
func NewKey[T any](name string) Key[T] {
	return Key[T]{
		id:   keyCounter.Add(1),
		name: name,
	}
}

// String returns a diagnostic string representation of the key.
func (k Key[T]) String() string {
	return k.name
}

// Set stores a typed value in the Context under the given key.
func Set[T any](ctx *Context, key Key[T], value T) {
	if ctx == nil {
		return
	}
	ctx.setTyped(key.id, value)
}

// Get retrieves a typed value from the Context under the given key.
// Returns the zero value and false if the key is not present or the
// stored value is not of type T.
func Get[T any](ctx *Context, key Key[T]) (T, bool) {
	if ctx == nil {
		var zero T
		return zero, false
	}
	v, ok := ctx.getTyped(key.id)
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	if !ok {
		var zero T
		return zero, false
	}
	return t, true
}
