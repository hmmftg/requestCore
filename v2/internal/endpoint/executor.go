package endpoint

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hmmftg/requestCore/v2/binding"
	"github.com/hmmftg/requestCore/v2/operation"
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

// WithProblemMapper sets the problem mapper registry.
func WithProblemMapper(m *response.MapperRegistry) ExecutorOption {
	return func(e *Executor) { e.ProblemMapper = m }
}

// NewExecutor creates a new Executor with the given options. Defaults:
// - Registry: operation.NewDefaultRegistry()
// - Telemetry: telemetry.NopSink{}
// - ProblemMapper: response.DefaultMapperRegistry()
func NewExecutor(opts ...ExecutorOption) *Executor {
	e := &Executor{
		Registry:      operation.NewRegistry(),
		Telemetry:     telemetry.NopSink{},
		ProblemMapper: response.DefaultMapperRegistry(),
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
			e.writeProblem(transport, problem)
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, err, start)
			return zero, err
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
			e.writeProblem(transport, problem)
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, err, start)
			return zero, err
		}
		if len(violations) > 0 {
			problem := response.NewValidationProblem(
				http.StatusUnprocessableEntity,
				"Validation Failed",
				violations,
			)
			e.writeProblem(transport, problem)
			e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, problem, start)
			return zero, problem
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
		e.writeProblem(transport, problem)
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, err, start)
		return zero, err
	}

	// 4. Encode
	body, err := json.Marshal(resp)
	if err != nil {
		problem := response.NewProblemWithCode(
			http.StatusInternalServerError,
			"Response Encoding Error",
			"ENCODE_ERROR",
		)
		e.writeProblem(transport, problem)
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, err, start)
		return zero, err
	}

	// 5-9. Commit lifecycle via the response state machine.
	cm := response.NewCommitMachine()
	if err := cm.Prepare(successStatus); err != nil {
		return zero, err
	}
	if err := cm.RunHooks(ctx.BeforeCommitHooks()); err != nil {
		problem := response.NewProblemWithCode(
			http.StatusInternalServerError,
			"Pre-commit Hook Failed",
			"HOOK_ERROR",
		)
		e.writeProblem(transport, problem)
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, problem.Status, err, start)
		return zero, err
	}
	if err := cm.MarkDurable(); err != nil {
		return zero, err
	}
	if err := cm.Commit(successStatus); err != nil {
		return zero, err
	}

	// Write the success response to the transport.
	if werr := transport.WriteResponse(successStatus, contentType, body); werr != nil {
		e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventFailure, http.StatusInternalServerError, werr, start)
		return zero, werr
	}

	if err := cm.Observe(); err != nil {
		return zero, err
	}

	e.emit(ep.Config.Operation.ID, ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventSuccess, successStatus, nil, start)
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

// writeProblem writes a Problem response to the transport as JSON.
func (e *Executor) writeProblem(transport Transport, problem *response.Problem) {
	if transport.Committed() {
		return
	}
	body, err := json.Marshal(problem)
	if err != nil {
		// Last resort: write a minimal 500.
		transport.WriteResponse(
			http.StatusInternalServerError,
			"application/json",
			[]byte(`{"title":"Internal Server Error","status":500}`),
		)
		return
	}
	transport.WriteResponse(problem.Status, response.ProblemContentType, body)
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
