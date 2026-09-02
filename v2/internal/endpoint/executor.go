package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
	"github.com/hmmftg/requestCore/v2/renderers"
	"github.com/hmmftg/requestCore/v2/request"
	"github.com/hmmftg/requestCore/v2/response"
	"github.com/hmmftg/requestCore/v2/telemetry"
)

// Executor runs the full endpoint lifecycle: bind → validate →
// execute → encode → prepare → hooks → durable → commit → observe.
type Executor struct {
	Registry      operation.Registry
	Telemetry     telemetry.Sink
	ProblemMapper *response.MapperRegistry
	Encoder       Encoder
}

// ExecutorOption configures an Executor at construction time.
type ExecutorOption func(*Executor)

// WithRegistry sets the operation registry.
func WithRegistry(r operation.Registry) ExecutorOption {
	return func(e *Executor) { e.Registry = r }
}

// WithTelemetrySink sets the telemetry sink.
func WithTelemetrySink(s telemetry.Sink) ExecutorOption {
	return func(e *Executor) { e.Telemetry = s }
}

// WithNopTelemetry sets the telemetry sink to NopSink, suppressing all
// telemetry output. This is intended for tests and benchmarks.
func WithNopTelemetry() ExecutorOption {
	return func(e *Executor) { e.Telemetry = telemetry.NopSink{} }
}

// WithProblemMapper sets the problem mapper registry.
func WithProblemMapper(m *response.MapperRegistry) ExecutorOption {
	return func(e *Executor) { e.ProblemMapper = m }
}

// WithExecutorEncoder sets the default encoder for the executor.
// Endpoints without an explicit Encoder use this. If nil, JSONRenderer
// is used.
func WithExecutorEncoder(enc Encoder) ExecutorOption {
	return func(e *Executor) { e.Encoder = enc }
}

// NewExecutor creates a new Executor with the given options. Defaults:
// - Registry: operation.NewRegistry()
// - Telemetry: telemetry.NewSlogSink(nil) (observable by default)
// - ProblemMapper: response.DefaultMapperRegistry()
// - Encoder: renderers.JSONRenderer{}
//
// Tests and benchmarks should use WithNopTelemetry() to suppress output.
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		Registry:      operation.NewRegistry(),
		Telemetry:     telemetry.NewSlogSink(nil),
		ProblemMapper: response.DefaultMapperRegistry(),
		Encoder:       renderers.JSONRenderer{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Register registers an endpoint's operation metadata with the
// registry. It detects duplicate operation IDs.
//
// Register is a generic standalone function because Go does not allow
// generic methods on non-generic types.
func Register[Req, Resp any](e *Executor, ep *Endpoint[Req, Resp]) error {
	return e.Registry.Register(ep.Config.Operation)
}

// Execute runs the full lifecycle for the given endpoint against the
// provided transport. It returns the response value and error from the
// handler, or an error from binding/validation/encoding.
//
// On any error, a Problem is mapped and written to the transport.
// Telemetry events are emitted on every branch (start, success,
// failure).
//
// Commit ordering on the success path is:
// Prepare → RunHooks → MarkDurable → transport.WriteResponse →
// Commit → Observe. The commit machine only transitions to Committed
// after the transport write succeeds; a transport failure transitions
// it to Failed.
//
// Execute is a generic standalone function because Go does not allow
// generic methods on non-generic types.
func Execute[Req, Resp any](e *Executor, ctx *request.Context, ep *Endpoint[Req, Resp], transport Transport) (Resp, error) {
	start := time.Now()
	var zero Resp

	successStatus := ep.Config.SuccessStatus
	if successStatus == 0 {
		successStatus = http.StatusOK
	}
	contentType := ep.Config.SuccessContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Emit start event.
	e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventStart, 0, nil, start)

	// 1. Bind
	var req Req
	if ep.Config.BindingPlan.Mode != binding.ModeNone {
		if err := binding.Bind(ctx, ep.Config.BindingPlan, &req); err != nil {
			problem := e.mapBindError(err)
			werr := e.writeProblem(ctx, transport, problem)
			finalErr := err
			if werr != nil {
				finalErr = fmt.Errorf("%w (problem write failed: %v)", err, werr)
			}
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
			return zero, finalErr
		}
	}

	// 2. Validate
	if ep.Config.Validator != nil {
		violations, err := ep.Config.Validator.Validate(&req)
		if err != nil {
			problem := response.NewProblemWithCode(
				http.StatusInternalServerError,
				"Validation Internal Error",
				"VALIDATION_INTERNAL",
			)
			werr := e.writeProblem(ctx, transport, problem)
			finalErr := err
			if werr != nil {
				finalErr = fmt.Errorf("%w (problem write failed: %v)", err, werr)
			}
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
			return zero, finalErr
		}
		if len(violations) > 0 {
			problem := response.NewValidationProblem(
				http.StatusUnprocessableEntity,
				"Validation Failed",
				violations,
			)
			werr := e.writeProblem(ctx, transport, problem)
			finalErr := error(problem)
			if werr != nil {
				finalErr = fmt.Errorf("%w (problem write failed: %v)", problem, werr)
			}
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
			return zero, finalErr
		}
	}

	// 3. Execute handler (with panic recovery)
	resp, err := executeHandler(ctx, ep, &req)
	if err != nil {
		problem := e.ProblemMapper.Map(err)
		if problem == nil {
			problem = response.NewProblemWithCode(
				http.StatusInternalServerError,
				"Internal Server Error",
				"INTERNAL",
			)
		}
		werr := e.writeProblem(ctx, transport, problem)
		finalErr := err
		if werr != nil {
			finalErr = fmt.Errorf("%w (problem write failed: %v)", err, werr)
		}
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
		return zero, finalErr
	}

	// 4. Encode — use endpoint encoder if set, otherwise executor default.
	encoder := ep.Config.Encoder
	if encoder == nil {
		encoder = e.Encoder
	}
	if encoder == nil {
		encoder = renderers.JSONRenderer{}
	}
	body, err := encoder.Encode(resp)
	if err != nil {
		problem := response.NewProblemWithCode(
			http.StatusInternalServerError,
			"Response Encoding Error",
			"ENCODE_ERROR",
		)
		werr := e.writeProblem(ctx, transport, problem)
		finalErr := err
		if werr != nil {
			finalErr = fmt.Errorf("%w (problem write failed: %v)", err, werr)
		}
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
		return zero, finalErr
	}
	// Derive content type from the encoder if the endpoint didn't
	// specify one explicitly.
	if contentType == "application/json" && ep.Config.SuccessContentType == "" {
		contentType = encoder.ContentType()
	}

	// Compute final success status and headers from the response state.
	// The endpoint-configured status wins unless the handler explicitly
	// set a status via ctx.Response().SetStatus.
	finalStatus := successStatus
	if ctx.Response().StatusSet() {
		finalStatus = ctx.Response().Status()
	}
	finalHeaders := ctx.Response().Header()
	finalContentType := contentType
	if ct := finalHeaders.Get("Content-Type"); ct != "" {
		finalContentType = ct
	}

	// 5-9. Commit lifecycle via the centralized commit coordinator.
	// The coordinator runs hooks exactly once, snapshots headers after
	// hooks, writes through the transport, and manages the commit state
	// machine. Both success and error paths flow through it.
	coordinator := response.NewCommitCoordinator()
	if err := coordinator.CommitSuccess(ctx, transport, finalStatus, finalContentType, body); err != nil {
		// If the coordinator wrote a Problem (hook failure), the
		// transport is already committed. Emit failure telemetry.
		if coordinator.State() == response.StateFailed {
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, http.StatusInternalServerError, err, start)
		}
		return zero, err
	}

	e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventSuccess, finalStatus, nil, start)
	return resp, nil
}

// executeHandler runs the handler with panic recovery.
func executeHandler[Req, Resp any](ctx *request.Context, ep *Endpoint[Req, Resp], req *Req) (resp Resp, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return ep.Handler(ctx, *req)
}

// mapBindError converts a binding error into a Problem.
func (e *Executor) mapBindError(err error) *response.Problem {
	var be *binding.BindingError
	if errors.As(err, &be) {
		return response.NewProblemWithCode(
			be.HTTPStatus(),
			fmt.Sprintf("Binding Error: %s", be.Cause),
			"BINDING_ERROR",
		)
	}
	return response.NewProblemWithCode(
		http.StatusBadRequest,
		"Bad Request",
		"BINDING_ERROR",
	)
}

// writeProblem writes a Problem response to the transport via the
// commit coordinator, preserving response headers from the context
// (especially Set-Cookie). It returns any transport or encoding error
// so callers can propagate it alongside the original domain error. If
// the transport is already committed, no write is attempted and nil is
// returned.
func (e *Executor) writeProblem(ctx *request.Context, transport Transport, problem *response.Problem) error {
	coordinator := response.NewCommitCoordinator()
	return coordinator.CommitProblem(ctx, transport, problem)
}

// emit sends a telemetry event if a sink is configured.
func (e *Executor) emit(opID, method, pattern string, ctx *request.Context, etype telemetry.EventType, status int, err error, start time.Time) {
	if e.Telemetry == nil {
		return
	}
	e.Telemetry.Record(telemetry.Event{
		Type:         etype,
		Operation:    opID,
		Method:       method,
		RoutePattern: pattern,
		Status:       status,
		Duration:     time.Since(start),
		RequestID:    ctx.RequestID(),
		TraceID:      ctx.TraceID(),
		Err:          err,
	})
}
