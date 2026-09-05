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
	otel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Executor runs the full endpoint lifecycle: bind → validate →
// (ID parse) → (tracer.StartSpan) → (persister.BeforeExecute) →
// (initializer) → execute → encode → (tracer.SetAttrs) → prepare →
// hooks → durable → commit → (persister.AfterCommit) → (finalizer) →
// (tracer.EndSpan) → observe.
type Executor struct {
	Registry      operation.Registry
	Telemetry     telemetry.Sink
	ProblemMapper *response.MapperRegistry
	Encoder       Encoder
	Tracer        oteltrace.Tracer
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

// WithExecutorTracer sets the default OpenTelemetry tracer for the
// executor. Endpoints without an explicit Tracer use this. If nil,
// a no-op tracer is used.
func WithExecutorTracer(t oteltrace.Tracer) ExecutorOption {
	return func(e *Executor) { e.Tracer = t }
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
// Full lifecycle:
//
//	bind → validate → (ID parse) → (span start) →
//	(persister.BeforeExecute) → (initializer) →
//	execute → encode → (span attrs) → prepare → hooks → durable →
//	commit → (persister.AfterCommit) → (finalizer) → (span end) →
//	observe.
//
// Execute is a generic standalone function because Go does not allow
// generic methods on non-generic types.
func Execute[Req, Resp any](e *Executor, ctx *request.Context, ep *Endpoint[Req, Resp], transport Transport) (Resp, error) {
	start := time.Now()
	var zero Resp
	opID := ep.Config.Operation.ID
	method := ep.Config.Operation.Method
	pattern := ep.Config.Operation.Pattern

	successStatus := ep.Config.SuccessStatus
	if successStatus == 0 {
		successStatus = http.StatusOK
	}
	contentType := ep.Config.SuccessContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// Emit start event with canonical <opID>-req operation key.
	e.emit(opID+"-req", method, pattern, ctx, telemetry.EventStart, 0, nil, start)

	// --- Tracing: start span ---
	var span oteltrace.Span
	spanCtx := ctx.Context()
	if ep.Config.EnableTracing {
		tracer := ep.Config.Tracer
		if tracer == nil {
			tracer = e.Tracer
		}
		if tracer == nil {
			tracer = otel.GetTracerProvider().Tracer("requestcore/v2")
		}
		spanName := ep.Config.TracingSpanName
		if spanName == "" {
			spanName = opID
		}
		spanCtx, span = tracer.Start(spanCtx, spanName,
			oteltrace.WithAttributes(
				attribute.String("operation.id", opID),
				attribute.String("operation.method", method),
				attribute.String("operation.pattern", pattern),
			),
		)
		// Update the context's inner context so handlers see the span.
		ctx.SetContext(spanCtx)
		if span.SpanContext().HasTraceID() {
			ctx.SetTraceID(span.SpanContext().TraceID().String())
		}
	}
	defer func() {
		if span != nil {
			span.End()
		}
	}()

	// Declare req early so the fail closure can reference it.
	var req Req

	// Helper for error path: write problem, emit failure, run finalizer.
	fail := func(err error, problem *response.Problem) (Resp, error) {
		if span != nil && problem != nil {
			span.SetAttributes(attribute.Int("http.status_code", problem.Status))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		werr := e.writeProblem(ctx, transport, problem)
		finalErr := err
		if werr != nil {
			finalErr = fmt.Errorf("%w (problem write failed: %v)", err, werr)
		}
		e.emit(opID+"-req-failed", method, pattern, ctx, telemetry.EventFailure, problem.Status, finalErr, start)
		// Run finalizer on error path.
		e.runFinalizer(ctx, ep, &req, &zero, finalErr)
		return zero, finalErr
	}

	// 1. Bind
	if ep.Config.BindingPlan.Mode != binding.ModeNone {
		if err := binding.Bind(ctx, ep.Config.BindingPlan, &req); err != nil {
			problem := e.mapBindError(err)
			return fail(err, problem)
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
			return fail(err, problem)
		}
		if len(violations) > 0 {
			problem := response.NewValidationProblem(
				http.StatusUnprocessableEntity,
				"Validation Failed",
				violations,
			)
			return fail(error(problem), problem)
		}
	}

	// 2b. ID Parser (after binding, before handler)
	if ep.Config.IDParser != nil {
		if err := ep.Config.IDParser(ctx); err != nil {
			problem := response.NewProblemWithCode(
				http.StatusBadRequest,
				"Invalid ID Parameter",
				"INVALID_ID",
			)
			return fail(err, problem)
		}
	}

	// 3. Persister.BeforeExecute (after validation/ID parse, before handler)
	if ep.Persister != nil {
		if err := ep.Persister.BeforeExecute(ctx, &req); err != nil {
			problem := e.ProblemMapper.Map(err)
			if problem == nil {
				problem = response.NewProblemWithCode(
					http.StatusInternalServerError,
					"Persistence Error",
					"PERSISTENCE_ERROR",
				)
			}
			return fail(err, problem)
		}
	}

	// 4. Initializer (after persistence, before handler)
	if ep.Initializer != nil {
		if err := ep.Initializer(ctx, &req); err != nil {
			problem := e.ProblemMapper.Map(err)
			if problem == nil {
				problem = response.NewProblemWithCode(
					http.StatusInternalServerError,
					"Initialization Error",
					"INIT_ERROR",
				)
			}
			return fail(err, problem)
		}
	}

	// 5. Execute handler (with panic recovery)
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
		return fail(err, problem)
	}

	// 6. Encode
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
		return fail(err, problem)
	}
	if contentType == "application/json" && ep.Config.SuccessContentType == "" {
		contentType = encoder.ContentType()
	}

	// Compute final success status and headers.
	finalStatus := successStatus
	if ctx.Response().StatusSet() {
		finalStatus = ctx.Response().Status()
	}
	finalHeaders := ctx.Response().Header()
	finalContentType := contentType
	if ct := finalHeaders.Get("Content-Type"); ct != "" {
		finalContentType = ct
	}

	// 7-11. Commit lifecycle via the centralized commit coordinator.
	coordinator := response.NewCommitCoordinator()
	if err := coordinator.CommitSuccess(ctx, transport, finalStatus, finalContentType, body); err != nil {
		if coordinator.State() == response.StateFailed {
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			e.emit(opID+"-req-failed", method, pattern, ctx, telemetry.EventFailure, http.StatusInternalServerError, err, start)
			e.runFinalizer(ctx, ep, &req, &zero, err)
		}
		return zero, err
	}

	// 12. Persister.AfterCommit (best-effort, errors logged not propagated)
	if ep.Persister != nil {
		if perr := ep.Persister.AfterCommit(ctx, &req, &resp, nil); perr != nil {
			e.emit(opID+"-req", method, pattern, ctx, telemetry.EventBusiness, finalStatus, perr, start)
		}
	}

	// 13. Finalizer (after commit, success path)
	e.runFinalizer(ctx, ep, &req, &resp, nil)

	// Tracing: set success attributes
	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", finalStatus))
		span.SetStatus(codes.Ok, "")
	}

	e.emit(opID+"-req", method, pattern, ctx, telemetry.EventSuccess, finalStatus, nil, start)
	return resp, nil
}

// runFinalizer runs the finalizer hook if set. Errors are logged via
// telemetry but not propagated. This is called on both success and
// error paths.
func (e *Executor) runFinalizer[Req, Resp any](ctx *request.Context, ep *Endpoint[Req, Resp], req *Req, resp *Resp, err error) {
	if ep.Finalizer == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			e.emit(ep.Config.Operation.ID+"-req", ep.Config.Operation.Method, ep.Config.Operation.Pattern, ctx, telemetry.EventBusiness, 0, fmt.Errorf("finalizer panic: %v", r), time.Now())
		}
	}()
	ep.Finalizer(ctx, req, resp, err)
}

// executeHandler runs the handler with panic recovery. If a custom
// RecoveryHandler is configured, it is called with the panic value.
func executeHandler[Req, Resp any](ctx *request.Context, ep *Endpoint[Req, Resp], req *Req) (resp Resp, err error) {
	defer func() {
		if r := recover(); r != nil {
			if ep.Config.RecoveryHandler != nil {
				if recErr := ep.Config.RecoveryHandler(r); recErr != nil {
					err = recErr
				} else {
					err = fmt.Errorf("handler panic: %v", r)
				}
			} else {
				err = fmt.Errorf("handler panic: %v", r)
			}
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
