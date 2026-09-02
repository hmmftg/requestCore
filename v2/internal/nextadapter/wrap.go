package nextadapter

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/v2/internal/endpoint"
	"github.com/hmmftg/requestCore/v2/routing"
	"github.com/hmmftg/requestCore/v2/telemetry"
	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
	legacy "github.com/hmmftg/requestCore/webFramework"
)

// Wrap converts a new-kernel internal endpoint into the existing
// v2-alpha routing.Handler contract. The returned handler:
//
//  1. Builds a new request.Context from the alpha RequestContext.
//  2. Starts transaction lifecycle logging via addWebLogs.
//  3. Shallow-copies the shared executor and installs a request-scoped
//     AddLogSink; the shared executor is never mutated.
//  4. Builds a parserTransport and calls endpoint.Execute.
//  5. On success, writes the mandatory <operation>-req AddLog entry
//     containing the masked typed response.
//  6. Completes transaction logging exactly once with the final status.
//  7. Returns nil if the alpha response is committed (so router error
//     dispatch cannot double-write); otherwise returns the error.
//
// Bridge failures that occur before executor telemetry starts emit
// <operation>-req-failed directly.
func Wrap[Req, Resp any](ep *endpoint.Endpoint[Req, Resp], exec *endpoint.Executor) routing.Handler {
	return func(rc *v2wf.RequestContext) error {
		start := time.Now()
		opID := ep.Config.Operation.ID

		if err := validateWrapInputs(ep, exec, rc); err != nil {
			emitBridgeFailure(rc, opID, err)
			return err
		}

		w := rc.Legacy
		if w.Parser == nil {
			err := errors.New("nextadapter: nil legacy parser")
			emitBridgeFailure(rc, opID, err)
			return err
		}

		// Start transaction lifecycle logging.
		logCompletion := addWebLogs(w, opID)

		// Build the new request context.
		reqCtx, err := buildContext(rc, ep.Config.Operation.Pattern, ep.Config.BindingPlan)
		if err != nil {
			emitBridgeFailure(rc, opID, err)
			logCompletion(start, http.StatusInternalServerError)
			return err
		}

		// Shallow-copy the executor for this request and install a
		// request-scoped AddLogSink. The shared executor is never
		// mutated.
		reqExec := *exec
		reqExec.Telemetry = &addLogSink{w: w, operation: opID}

		transport := &parserTransport{rc: rc}

		// Run the internal endpoint executor.
		resp, execErr := endpoint.Execute(&reqExec, reqCtx, ep, transport)

		// Determine the final committed status for transaction logging.
		finalStatus := http.StatusOK
		if rc.Committed() {
			if cs := rc.CommitState(); cs != nil {
				finalStatus = cs.Status()
			}
		} else if execErr != nil {
			finalStatus = http.StatusInternalServerError
		}

		if execErr != nil {
			// The executor already wrote a problem response and emitted
			// failure telemetry. Complete transaction logging and
			// prevent router double-write if committed.
			logCompletion(start, finalStatus)
			if rc.Committed() {
				return nil
			}
			return execErr
		}

		// Success: emit the mandatory <operation>-req AddLog entry
		// containing the masked typed response.
		legacy.AddLog(w, opID+"-req", slog.Any("response", safeLogValue(resp)))

		logCompletion(start, finalStatus)

		// If the alpha response is committed, return nil so router
		// error dispatch cannot double-write.
		if rc.Committed() {
			return nil
		}
		return nil
	}
}

// validateWrapInputs checks the non-nil preconditions for Wrap.
func validateWrapInputs[Req, Resp any](ep *endpoint.Endpoint[Req, Resp], exec *endpoint.Executor, rc *v2wf.RequestContext) error {
	if ep == nil {
		return errors.New("nextadapter: nil endpoint")
	}
	if exec == nil {
		return errors.New("nextadapter: nil executor")
	}
	if rc == nil {
		return errors.New("nextadapter: nil RequestContext")
	}
	if rc.Parser == nil {
		return errors.New("nextadapter: nil Parser")
	}
	return nil
}

// emitBridgeFailure emits a <operation>-req-failed AddLog entry for a
// bridge failure that occurs before executor telemetry starts.
func emitBridgeFailure(rc *v2wf.RequestContext, opID string, err error) {
	if rc == nil || rc.Legacy.Parser == nil {
		return
	}
	legacy.AddLog(rc.Legacy, opID+"-req-failed", slog.Any("error", err))
}

// Register registers a new-kernel endpoint's operation metadata with
// the executor's registry and registers the wrapped handler on the
// alpha route group.
//
// Pre-validation removes expected failures before mutation:
//   - non-nil dependencies;
//   - canonical route pattern;
//   - operation ID, method, and pattern explicitly present;
//   - operation method/pattern match the supplied registration
//     method/path (immutable metadata is validated, not rewritten).
//
// Residual limitation: the current registry/router contracts do not
// support rollback if router registration fails after metadata
// registration. Pre-validation removes expected failures before
// mutation, but a router Handle failure after a successful Register
// leaves the metadata registered.
func Register[Req, Resp any](
	router routing.RouteGroup,
	exec *endpoint.Executor,
	method, path string,
	ep *endpoint.Endpoint[Req, Resp],
) error {
	if router == nil {
		return errors.New("nextadapter: nil router")
	}
	if exec == nil {
		return errors.New("nextadapter: nil executor")
	}
	if ep == nil {
		return errors.New("nextadapter: nil endpoint")
	}
	if err := routing.ValidatePattern(path); err != nil {
		return fmt.Errorf("nextadapter: invalid pattern: %w", err)
	}
	op := ep.Config.Operation
	if op.ID == "" {
		return errors.New("nextadapter: operation ID is required")
	}
	if op.Method == "" {
		return errors.New("nextadapter: operation method is required")
	}
	if op.Pattern == "" {
		return errors.New("nextadapter: operation pattern is required")
	}
	if op.Method != method {
		return fmt.Errorf("nextadapter: operation method %q does not match registration method %q", op.Method, method)
	}
	if op.Pattern != path {
		return fmt.Errorf("nextadapter: operation pattern %q does not match registration path %q", op.Pattern, path)
	}

	// Register operation metadata first.
	if err := endpoint.Register(exec, ep); err != nil {
		return fmt.Errorf("nextadapter: register operation: %w", err)
	}

	// Register the wrapped handler on the alpha route group.
	if err := router.Handle(method, path, Wrap[Req, Resp](ep, exec)); err != nil {
		return fmt.Errorf("nextadapter: register route: %w", err)
	}
	return nil
}

// Compile-time checks.
var (
	_ telemetry.Sink = (*addLogSink)(nil)
)
