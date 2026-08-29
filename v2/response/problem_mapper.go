package response

import (
	"errors"
	"net/http"
	"sync"
)

// Mapper converts an error into an RFC 9457 Problem. Custom mappers
// return a complete Problem, not raw response bytes, preserving
// central commit and content type.
//
// If the mapper does not match the error, it should return nil so the
// registry can try the next mapper or the fallback.
type Mapper func(err error) *Problem

// Matcher determines whether an error should be handled by a specific
// mapper. It typically uses errors.As to inspect the error chain.
// Returns true if the mapper should handle this error.
type Matcher func(err error) bool

// MapperRegistry holds a set of error-to-Problem mappers with matchers.
// When Map is called, the registry checks each registered matcher in
// order; the first matching mapper wins. If no mapper matches, the
// default sanitizer produces a 500 problem.
//
// Unknown errors always become sanitized 500 problems. Causes are
// never serialized by default.
type MapperRegistry struct {
	mu       sync.RWMutex
	entries  []mapperEntry
	fallback Mapper
}

type mapperEntry struct {
	matcher Matcher
	mapper  Mapper
}

// NewMapperRegistry creates an empty MapperRegistry with a default
// 500 sanitizer fallback.
func NewMapperRegistry() *MapperRegistry {
	return &MapperRegistry{
		fallback: defaultSanitizerMapper,
	}
}

// DefaultMapperRegistry returns a registry with the default 500
// sanitizer as the fallback. This is the standard registry used by
// canonical applications.
func DefaultMapperRegistry() *MapperRegistry {
	return NewMapperRegistry()
}

// Register associates a mapper with a matcher. When Map encounters an
// error that matches the matcher, the mapper is invoked. Registration
// order matters: the first matching mapper wins.
//
// Returns an error if matcher or mapper is nil.
//
// Example:
//
//	r.Register(func(err error) bool {
//	    var e *MyError
//	    return errors.As(err, &e)
//	}, func(err error) *Problem {
//	    var e *MyError
//	    errors.As(err, &e)
//	    return NewProblemWithCode(409, "Conflict", "MY_ERROR").WithDetail(e.msg)
//	})
func (r *MapperRegistry) Register(matcher Matcher, mapper Mapper) error {
	if matcher == nil {
		return errors.New("problem: nil matcher")
	}
	if mapper == nil {
		return errors.New("problem: nil mapper")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, mapperEntry{matcher: matcher, mapper: mapper})
	return nil
}

// SetFallback sets the fallback mapper invoked when no registered
// mapper matches. If nil is passed, the default sanitizer is used.
func (r *MapperRegistry) SetFallback(mapper Mapper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mapper == nil {
		mapper = defaultSanitizerMapper
	}
	r.fallback = mapper
}

// Map converts an error into a Problem. It checks registered mappers
// in order; the first matching mapper wins. If no mapper matches,
// the fallback sanitizer produces a 500 problem.
//
// If err is nil, Map returns nil.
// If err is already a *Problem, Map returns it as-is.
func (r *MapperRegistry) Map(err error) *Problem {
	if err == nil {
		return nil
	}
	// If already a Problem, return as-is.
	var p *Problem
	if errors.As(err, &p) {
		return p
	}

	r.mu.RLock()
	entries := r.entries
	fallback := r.fallback
	r.mu.RUnlock()

	for _, entry := range entries {
		if entry.matcher(err) {
			if p := entry.mapper(err); p != nil {
				return p
			}
		}
	}
	return fallback(err)
}

// defaultSanitizerMapper converts any unknown error into a sanitized
// 500 problem. The error detail is never exposed; only a generic
// "Internal Server Error" is returned.
func defaultSanitizerMapper(err error) *Problem {
	return NewProblemWithCode(
		http.StatusInternalServerError,
		"Internal Server Error",
		"INTERNAL",
	)
}
