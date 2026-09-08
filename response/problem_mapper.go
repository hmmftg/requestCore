package response

import (
	"errors"
	"net/http"
	"sync"

	"github.com/hmmftg/requestCore/libError"
)

// ProblemMapper converts an error into an RFC 9457 Problem. Custom
// mappers return a complete Problem, not raw response bytes, preserving
// central commit and content type.
//
// If the mapper does not match the error, it should return nil so the
// registry can try the next mapper or the fallback.
type ProblemMapper func(err error) *Problem

// ProblemMatcher determines whether an error should be handled by a
// specific mapper. It typically uses errors.As to inspect the error
// chain. Returns true if the mapper should handle this error.
type ProblemMatcher func(err error) bool

// ProblemMapperRegistry holds a set of error-to-Problem mappers with
// matchers. When Map is called, the registry checks each registered
// matcher in order; the first matching mapper wins. If no mapper
// matches, the default sanitizer produces a 500 problem.
//
// Unknown errors always become sanitized 500 problems. Causes are
// never serialized by default.
//
// Once frozen, Register and SetFallback return ErrRegistryFrozen. The
// registry should be frozen before serving to prevent runtime mutation.
type ProblemMapperRegistry struct {
	mu       sync.RWMutex
	entries  []problemMapperEntry
	fallback ProblemMapper
	frozen   bool
}

// ErrProblemRegistryFrozen is returned when attempting to register or
// modify a frozen ProblemMapperRegistry.
var ErrProblemRegistryFrozen = errors.New("response: problem mapper registry is frozen")

type problemMapperEntry struct {
	matcher ProblemMatcher
	mapper  ProblemMapper
}

// NewProblemMapperRegistry creates an empty ProblemMapperRegistry with
// a default 500 sanitizer fallback.
func NewProblemMapperRegistry() *ProblemMapperRegistry {
	return &ProblemMapperRegistry{
		fallback: defaultProblemSanitizerMapper,
	}
}

// DefaultProblemMapperRegistry returns a registry with the default 500
// sanitizer as the fallback, plus built-in mappers for libError.ErrorData
// and response.ErrorData. This is the standard registry for v1
// applications that opt into RFC 9457 error responses.
func DefaultProblemMapperRegistry() *ProblemMapperRegistry {
	r := NewProblemMapperRegistry()
	_ = r.Register(libErrorProblemMatcher, libErrorProblemMapper)
	_ = r.Register(errorDataProblemMatcher, errorDataProblemMapper)
	return r
}

// Register associates a mapper with a matcher. When Map encounters an
// error that matches the matcher, the mapper is invoked. Registration
// order matters: the first matching mapper wins.
//
// Returns an error if matcher or mapper is nil.
func (r *ProblemMapperRegistry) Register(matcher ProblemMatcher, mapper ProblemMapper) error {
	if matcher == nil {
		return errors.New("problem: nil matcher")
	}
	if mapper == nil {
		return errors.New("problem: nil mapper")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrProblemRegistryFrozen
	}
	r.entries = append(r.entries, problemMapperEntry{matcher: matcher, mapper: mapper})
	return nil
}

// SetFallback sets the fallback mapper invoked when no registered
// mapper matches. If nil is passed, the default sanitizer is used.
// Returns ErrProblemRegistryFrozen if the registry is frozen.
func (r *ProblemMapperRegistry) SetFallback(mapper ProblemMapper) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.frozen {
		return ErrProblemRegistryFrozen
	}
	if mapper == nil {
		mapper = defaultProblemSanitizerMapper
	}
	r.fallback = mapper
	return nil
}

// Freeze prevents further registration or fallback changes. Called
// after startup before serving requests.
func (r *ProblemMapperRegistry) Freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// Frozen reports whether the registry is frozen.
func (r *ProblemMapperRegistry) Frozen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}

// Map converts an error into a Problem. It checks registered mappers
// in order; the first matching mapper wins. If no mapper matches,
// the fallback sanitizer produces a 500 problem.
//
// If err is nil, Map returns nil.
// If err is already a *Problem, Map returns it as-is.
func (r *ProblemMapperRegistry) Map(err error) *Problem {
	if err == nil {
		return nil
	}
	// If already a Problem, return as-is.
	var p *Problem
	if errors.As(err, &p) {
		return p
	}

	r.mu.RLock()
	// Snapshot entries to avoid concurrent mutation during iteration.
	entries := append([]problemMapperEntry(nil), r.entries...)
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

// defaultProblemSanitizerMapper converts any unknown error into a
// sanitized 500 problem. The error detail is never exposed; only a
// generic "Internal Server Error" is returned.
func defaultProblemSanitizerMapper(err error) *Problem {
	return NewProblemWithCode(
		http.StatusInternalServerError,
		"Internal Server Error",
		"INTERNAL",
	)
}

// libErrorProblemMatcher matches libError.ErrorData errors.
func libErrorProblemMatcher(err error) bool {
	var e libError.ErrorData
	return errors.As(err, &e)
}

// libErrorProblemMapper maps libError.ErrorData to a Problem. It uses
// the ActionData.Description as the code, PublicDescription as detail
// when available, and the ActionData.Status as the HTTP status. The
// internal Message is never exposed.
func libErrorProblemMapper(err error) *Problem {
	var e libError.ErrorData
	if !errors.As(err, &e) {
		return nil
	}
	status := e.ActionData.Status.Int()
	if status == 0 {
		status = http.StatusInternalServerError
	}
	p := NewProblemWithCode(status, httpStatusTitle(status), e.ActionData.Description)
	if e.ActionData.PublicDescription != "" {
		p = p.WithDetail(SanitizeForClient(e.ActionData.PublicDescription, MaxDescriptionLength))
	}
	return p.WithCause(err)
}

// errorDataProblemMatcher matches response.ErrorData errors. Both
// pointer and value forms are matched since the codebase uses both
// (though pointers are the convention for response.ErrorData).
func errorDataProblemMatcher(err error) bool {
	var p *ErrorData
	if errors.As(err, &p) {
		return true
	}
	var v ErrorData
	return errors.As(err, &v)
}

// errorDataProblemMapper maps response.ErrorData to a Problem. It uses
// the Description as the code and Status as the HTTP status. The
// internal Message is never exposed.
func errorDataProblemMapper(err error) *Problem {
	var p *ErrorData
	if errors.As(err, &p) {
		return mapErrorDataToProblem(p, err)
	}
	var v ErrorData
	if errors.As(err, &v) {
		return mapErrorDataToProblem(&v, err)
	}
	return nil
}

func mapErrorDataToProblem(e *ErrorData, cause error) *Problem {
	status := e.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	problem := NewProblemWithCode(status, httpStatusTitle(status), e.Description)

	// If Message contains validation errors (ErrorResponse array), map
	// them to violations.
	if errs, ok := e.Message.([]ErrorResponse); ok && len(errs) > 0 {
		violations := make([]ProblemViolation, 0, len(errs))
		for _, er := range errs {
			violations = append(violations, ProblemViolation{
				Field:   er.Code,
				Message: SanitizeForClient(er.Description, MaxDescriptionLength),
			})
		}
		problem.Violations = violations
	}
	return problem.WithCause(cause)
}

// httpStatusTitle returns a standard HTTP status title for the given
// status code, falling back to "Error" for unknown codes.
func httpStatusTitle(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusUnauthorized:
		return "Unauthorized"
	case http.StatusForbidden:
		return "Forbidden"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusConflict:
		return "Conflict"
	case http.StatusUnprocessableEntity:
		return "Unprocessable Entity"
	case http.StatusTooManyRequests:
		return "Too Many Requests"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	case http.StatusNotImplemented:
		return "Not Implemented"
	case http.StatusBadGateway:
		return "Bad Gateway"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	case http.StatusGatewayTimeout:
		return "Gateway Timeout"
	case http.StatusPreconditionFailed:
		return "Precondition Failed"
	case http.StatusPreconditionRequired:
		return "Precondition Required"
	default:
		return "Error"
	}
}
