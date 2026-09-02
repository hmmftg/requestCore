package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hmmftg/requestCore/v2/request"
)

// Transport abstracts the response-writing side of an HTTP request. It
// is structurally compatible with endpoint.Transport and
// routing.Transport so that the same concrete transports satisfy all
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

// CommitCoordinator centralizes the response commit lifecycle. It
// ensures that before-commit hooks run exactly once, that response
// headers are snapshotted after hooks, and that both success and error
// responses flow through the same state-machine path.
//
// A single CommitCoordinator is created per request. It is NOT safe
// for concurrent use; a request is processed by one goroutine.
type CommitCoordinator struct {
	machine *CommitMachine
}

// NewCommitCoordinator creates a CommitCoordinator in the initial
// (Open) state.
func NewCommitCoordinator() *CommitCoordinator {
	return &CommitCoordinator{machine: NewCommitMachine()}
}

// Machine returns the underlying CommitMachine for state inspection.
func (c *CommitCoordinator) Machine() *CommitMachine { return c.machine }

// CommitSuccess runs the full success commit lifecycle:
//  1. Prepare with the final status.
//  2. Run before-commit hooks exactly once (via request.Context).
//  3. Snapshot response headers after hooks.
//  4. Mark durable.
//  5. Write the response through the transport (suppressing body for
//     no-body statuses and HEAD).
//  6. Commit and observe.
//
// If hooks fail, a HOOK_ERROR Problem is written via CommitProblem
// (which preserves response headers and does not re-run hooks).
//
// If the transport write fails, the machine transitions to Failed and
// the error is returned.
func (c *CommitCoordinator) CommitSuccess(
	ctx *request.Context,
	transport Transport,
	status int,
	contentType string,
	body []byte,
) error {
	if err := c.machine.Prepare(status); err != nil {
		return err
	}

	// Run hooks exactly once. The request.Context tracks idempotency
	// and is the single source for hook execution. The commit machine
	// is transitioned separately (with nil hooks) to advance state.
	if err := ctx.RunBeforeCommitHooks(); err != nil {
		// Hook failure: write a Problem via the error path, which
		// preserves response headers and does not re-run hooks.
		problem := NewProblemWithCode(
			http.StatusInternalServerError,
			"Pre-commit Hook Failed",
			"HOOK_ERROR",
		)
		_ = c.machine.Fail(err)
		_ = c.writeProblemPreservingHeaders(ctx, transport, problem)
		return err
	}

	// Transition the machine from Prepared to HooksRun. The actual
	// hooks were already executed via ctx.RunBeforeCommitHooks above;
	// passing nil avoids double execution.
	if err := c.machine.RunHooks(nil); err != nil {
		return err
	}

	if err := c.machine.MarkDurable(); err != nil {
		return err
	}

	// Snapshot headers AFTER hooks so hook-added headers (e.g.
	// Set-Cookie from session middleware) are included.
	headers := ctx.Response().Header()

	// Suppress body for no-body statuses and explicitly suppressed
	// responses.
	finalBody := body
	if request.IsNoBodyStatus(status) || ctx.Response().BodySuppressed() {
		finalBody = nil
	}

	// If the handler set a Content-Type header, it takes precedence.
	if ct := headers.Get("Content-Type"); ct != "" {
		contentType = ct
	}

	if werr := transport.WriteResponse(status, contentType, headers, finalBody); werr != nil {
		_ = c.machine.Fail(werr)
		return werr
	}

	if err := c.machine.Commit(status); err != nil {
		return err
	}
	return c.machine.Observe()
}

// CommitProblem writes a Problem response on an error path. It:
//  1. Runs before-commit hooks exactly once (so session save hooks fire
//     even on error paths).
//  2. Preserves response headers from the context (especially
//     Set-Cookie) by merging them with the Problem's headers.
//  3. Writes the Problem through the transport.
//  4. Transitions the machine to Failed.
//
// If the transport is already committed, no write is attempted.
// If hooks have already run (e.g. via a previous CommitSuccess call),
// they are not re-run.
func (c *CommitCoordinator) CommitProblem(
	ctx *request.Context,
	transport Transport,
	problem *Problem,
) error {
	return c.writeProblemPreservingHeaders(ctx, transport, problem)
}

// writeProblemPreservingHeaders writes a Problem response, merging
// response headers from the context (especially Set-Cookie) with the
// Problem's own headers. Hooks are run once via the context's
// idempotent RunBeforeCommitHooks. The machine transitions to Failed.
func (c *CommitCoordinator) writeProblemPreservingHeaders(
	ctx *request.Context,
	transport Transport,
	problem *Problem,
) error {
	if transport.Committed() {
		return nil
	}

	// Run hooks once (idempotent). On error paths, hook failures are
	// best-effort: we still try to write the Problem response.
	_ = ctx.RunBeforeCommitHooks()

	// Snapshot response headers after hooks to preserve Set-Cookie
	// and other headers set by middleware.
	ctxHeaders := ctx.Response().Header()

	body, err := json.Marshal(problem)
	if err != nil {
		// Last resort: write a minimal 500 with preserved headers.
		_ = c.machine.Fail(fmt.Errorf("problem encode: %w", err))
		return transport.WriteResponse(
			http.StatusInternalServerError,
			ProblemContentType,
			ctxHeaders,
			[]byte(`{"title":"Internal Server Error","status":500}`),
		)
	}

	_ = c.machine.Fail(problem)
	return transport.WriteResponse(problem.Status, ProblemContentType, ctxHeaders, body)
}

// State returns the current commit state machine state.
func (c *CommitCoordinator) State() State { return c.machine.State() }
