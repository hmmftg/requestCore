// Package handlers provides the v2 request handler lifecycle, typed endpoint
// descriptors, and resource primitives for requestCore applications.
//
// The v2 BaseHandler preserves the v1 lifecycle (parse → log → persist →
// initialize → handle → render → finalize → persist-update) while routing
// responses and errors through the v2 response handler and error registry.
// Mandatory webFramework.AddLog calls are emitted on every request, success
// or failure, so transactions flow into the Splunk pipeline.
package handlers

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/hmmftg/requestCore"
	"github.com/hmmftg/requestCore/libRequest"
	legacy "github.com/hmmftg/requestCore/webFramework"

	v2wf "github.com/hmmftg/requestCore/v2/webFramework"
)

// HandlerParameters holds configuration for a v2 request handler.
type HandlerParameters[Req, Resp any] struct {
	Title           string
	Body            libRequest.Type
	ValidateHeader  bool
	Path            string
	RecoveryHandler func(any)
	LogArrays       []string
	LogTags         []string
	Persistence     RequestPersister[Req, Resp]
	EnableTracing   bool
	TracingSpanName string
}

// HandlerOutcome holds the error and HTTP status result of a handler request.
type HandlerOutcome struct {
	Error      error
	HTTPStatus int
}

// HandlerRequest holds the per-request state passed through the v2 handler
// lifecycle. It composes the v2 RequestContext with typed request/response
// fields and tracing state.
type HandlerRequest[Req, Resp any] struct {
	Title    string
	Core     requestCore.RequestCoreInterface
	Header   *libRequest.RequestHeader
	Request  *Req
	Response Resp
	W        legacy.WebFramework
	V2       *v2wf.RequestContext
	Args     []any
	RespSent bool
	Outcome  HandlerOutcome
	Duration time.Duration
	Span     trace.Span
	SpanCtx  context.Context
}

// SetOutcome sets the error and HTTP status on the handler request's outcome.
func (trx *HandlerRequest[Req, Resp]) SetOutcome(err error, httpStatus int) {
	trx.Outcome.Error = err
	trx.Outcome.HTTPStatus = httpStatus
}

// RequestPersister persists request state before and after handler execution.
// Insert is called after successful parsing and before the initializer.
// Update is called best-effort in the finalizer after the response is sent.
type RequestPersister[Req, Resp any] interface {
	Insert(path string, trx *HandlerRequest[Req, Resp]) error
	Update(path string, trx *HandlerRequest[Req, Resp]) error
}

// PersisterFunc is a convenience type for implementing RequestPersister with
// function values.
type PersisterFunc[Req, Resp any] struct {
	InsertFn func(path string, trx *HandlerRequest[Req, Resp]) error
	UpdateFn func(path string, trx *HandlerRequest[Req, Resp]) error
}

// Insert calls the configured insert function.
func (p PersisterFunc[Req, Resp]) Insert(path string, trx *HandlerRequest[Req, Resp]) error {
	if p.InsertFn == nil {
		return nil
	}
	return p.InsertFn(path, trx)
}

// Update calls the configured update function.
func (p PersisterFunc[Req, Resp]) Update(path string, trx *HandlerRequest[Req, Resp]) error {
	if p.UpdateFn == nil {
		return nil
	}
	return p.UpdateFn(path, trx)
}
