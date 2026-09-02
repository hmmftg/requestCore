package response

import (
	"net/http"

	"github.com/hmmftg/requestCore/v2/request"
)

// NoContent writes a 204 No Content response through the transport,
// running before-commit hooks first. The response body is suppressed
// per HTTP semantics for 204 responses.
func NoContent(ctx *request.Context, transport Transport) error {
	cc := NewCommitCoordinator()
	return cc.CommitSuccess(ctx, transport, http.StatusNoContent, "", nil)
}

// Redirect writes a redirect response through the transport, running
// before-commit hooks first. The Location header is set on the
// response state before commit so the CommitCoordinator includes it
// in the header snapshot. The response body is suppressed for 301,
// 302, 303, 307, and 308 per HTTP semantics (clients follow the
// redirect without reading the body).
func Redirect(ctx *request.Context, transport Transport, status int, url string) error {
	if status < 300 || status > 399 {
		return ErrInvalidRedirectStatus
	}
	ctx.Response().AddHeader("Location", url)
	cc := NewCommitCoordinator()
	return cc.CommitSuccess(ctx, transport, status, "", nil)
}

// WriteSuccess writes a successful response through the transport,
// running before-commit hooks first. The content type and body are
// written as-is, except that body is suppressed for no-body statuses
// (204, 304) and HEAD requests (detected via response state).
func WriteSuccess(ctx *request.Context, transport Transport, status int, contentType string, body []byte) error {
	cc := NewCommitCoordinator()
	return cc.CommitSuccess(ctx, transport, status, contentType, body)
}

// WriteProblem writes a Problem response through the transport,
// running before-commit hooks first (best-effort on error paths).
// Response headers from the context (especially Set-Cookie) are
// preserved by merging them with the Problem's headers.
func WriteProblem(ctx *request.Context, transport Transport, problem *Problem) error {
	cc := NewCommitCoordinator()
	return cc.CommitProblem(ctx, transport, problem)
}

// WriteProblemFromError maps an error to a Problem via the given
// MapperRegistry and writes it through the transport. If the mapper
// returns nil, a generic 500 Problem is written instead.
func WriteProblemFromError(ctx *request.Context, transport Transport, mapper *MapperRegistry, err error) error {
	if err == nil {
		return nil
	}
	var problem *Problem
	if mapper != nil {
		problem = mapper.Map(err)
	}
	if problem == nil {
		problem = NewProblemWithCode(
			http.StatusInternalServerError,
			"Internal Server Error",
			"INTERNAL",
		)
	}
	return WriteProblem(ctx, transport, problem)
}

// ErrInvalidRedirectStatus is returned when Redirect is called with
// a status code outside the 3xx range.
var ErrInvalidRedirectStatus = errInvalidRedirectStatus{}

type errInvalidRedirectStatus struct{}

func (errInvalidRedirectStatus) Error() string {
	return "response: redirect status must be in the 3xx range"
}
