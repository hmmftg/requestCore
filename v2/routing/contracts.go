// Package routing provides framework-agnostic route groups, middleware,
// and a Router interface implemented by Gin, Fiber, and net/http+chi adapters.
//
// Canonical parameter syntax is {id}, which adapters translate to their
// framework-specific syntax (:id for Gin/Fiber, {id} for chi).
//
// routing imports only the request package from the v2 kernel. It does
// not import webFramework, response, endpoint, handlers, or any v1
// package.
package routing

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hmmftg/requestCore/v2/request"
)

// Transport abstracts the response-writing side of an HTTP request. It
// is structurally compatible with response.Transport and
// endpoint.Transport so that the same concrete transports satisfy all
// three interfaces without adapter wrappers.
type Transport interface {
	// WriteResponse writes the HTTP response with the given status,
	// content type, headers, and body. It is called exactly once per
	// request. Implementations must apply all header values (using add
	// semantics for multi-value headers like Set-Cookie) before writing
	// the status and body. An empty content type is ignored. A nil
	// headers map is treated as empty.
	WriteResponse(status int, contentType string, headers http.Header, body []byte) error

	// Committed reports whether the response has already been
	// committed to the wire. Once true, WriteResponse must not be
	// called again.
	Committed() bool
}

// Handler is a v2 route handler that receives a request.Context and a
// response Transport, and returns an error. If the handler returns a
// non-nil error, the error is routed through the configured
// ErrorHandler.
type Handler func(*request.Context, Transport) error

// Middleware wraps a Handler with additional behavior.
// Middleware functions are composed in registration order: the first
// registered middleware is the outermost wrapper.
type Middleware func(Handler) Handler

// ErrorHandler handles errors returned by route handlers and middleware.
// It receives the request context, the response transport, and the
// error. The error handler is responsible for writing an appropriate
// error response (typically an RFC 9457 Problem) through the transport.
type ErrorHandler func(*request.Context, Transport, error)

// RouteGroup represents a group of routes with a shared path prefix
// and middleware chain.
type RouteGroup interface {
	// Group creates a sub-group with the given prefix appended to the
	// current group's prefix. The new group inherits the current
	// group's middleware.
	Group(prefix string) RouteGroup

	// With returns a new group that adds the given middleware to the
	// current group's middleware chain. The original group is unchanged.
	With(middleware ...Middleware) RouteGroup

	// Handle registers a handler for the given method and pattern.
	// The pattern uses canonical syntax: {id} for path parameters.
	Handle(method, pattern string, handler Handler) error

	// Convenience methods for common HTTP methods.
	Get(pattern string, handler Handler) error
	Post(pattern string, handler Handler) error
	Put(pattern string, handler Handler) error
	Patch(pattern string, handler Handler) error
	Delete(pattern string, handler Handler) error
	Head(pattern string, handler Handler) error
}

// Router extends RouteGroup with not-found, method-not-allowed, and
// error-handling configuration.
type Router interface {
	RouteGroup

	// NotFound sets the handler for unmatched routes.
	NotFound(handler Handler)

	// MethodNotAllowed sets the handler for disallowed methods on
	// matched paths.
	MethodNotAllowed(handler Handler)

	// SetErrorHandler sets the error handler invoked when a route
	// handler or middleware returns a non-nil error. This replaces
	// the previous reflection-based error-handler wiring.
	SetErrorHandler(handler ErrorHandler)

	// Native returns the underlying framework-specific router object
	// (e.g. *gin.Engine, *fiber.App, *chi.Mux). This is an escape hatch
	// for framework-specific configuration.
	Native() any
}

// Chain composes multiple middleware into a single middleware.
// The first middleware in the slice is the outermost wrapper.
func Chain(middleware ...Middleware) Middleware {
	if len(middleware) == 0 {
		return nil
	}
	if len(middleware) == 1 {
		return middleware[0]
	}
	return func(next Handler) Handler {
		for i := len(middleware) - 1; i >= 0; i-- {
			next = middleware[i](next)
		}
		return next
	}
}

// TranslatePattern converts a canonical v2 pattern ({id}) to the
// framework-specific syntax. The target format is either "gin" (":id")
// or "chi" ("{id}").
func TranslatePattern(pattern, target string) string {
	switch target {
	case "gin", "fiber":
		// Replace {param} with :param
		result := pattern
		for {
			start := strings.Index(result, "{")
			if start == -1 {
				break
			}
			end := strings.Index(result[start:], "}")
			if end == -1 {
				break
			}
			end += start
			param := result[start+1 : end]
			result = result[:start] + ":" + param + result[end+1:]
		}
		return result
	case "chi", "nethttp":
		return pattern
	default:
		return pattern
	}
}

// ValidatePattern checks that a pattern uses valid canonical syntax.
// Returns an error if the pattern contains malformed parameter braces.
func ValidatePattern(pattern string) error {
	depth := 0
	for i, r := range pattern {
		switch r {
		case '{':
			if depth > 0 {
				return fmt.Errorf("routing: nested parameter in pattern %q at position %d", pattern, i)
			}
			depth++
		case '}':
			if depth == 0 {
				return fmt.Errorf("routing: unmatched closing brace in pattern %q at position %d", pattern, i)
			}
			depth--
		}
	}
	if depth > 0 {
		return fmt.Errorf("routing: unclosed parameter in pattern %q", pattern)
	}
	return nil
}

// JoinPath joins path segments with proper slash handling.
func JoinPath(base, rel string) string {
	if base == "" {
		return rel
	}
	if rel == "" {
		return base
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	if strings.HasPrefix(rel, "/") {
		rel = rel[1:]
	}
	return base + rel
}
