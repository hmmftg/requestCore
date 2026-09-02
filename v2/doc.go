// Package requestcore is the v2 module of requestCore.
//
// v2 is a typed, framework-neutral HTTP toolkit built on a canonical
// kernel. It requires Go 1.22+ for the net/http ServeMux pattern
// matching and Go 1.27+ for generic methods on typed endpoints.
//
// # Architecture
//
// The v2 kernel is organized as a layered DAG:
//
//   - request — stdlib-only request state, lazy body source, typed
//     values, response metadata, before-commit hooks.
//   - telemetry — stdlib-only event/sink contracts and slog sink.
//   - binding, validation, operation, renderers — leaf capabilities.
//   - response — Problems (RFC 9457), mapper registry, commit
//     coordinator, no-content/redirect helpers.
//   - endpoint — typed endpoint and executor; the canonical lifecycle
//     (bind → validate → execute → encode → commit → observe).
//   - routing — handler/middleware/router and response-transport
//     contracts; imports request only.
//   - adapter — adapts typed endpoints and mapped errors to routing
//     handlers.
//   - libGin, libFiber, libChi, libNetHttp — native context/transport
//     construction for each framework.
//   - handlers — convenience constructors and the non-generic runtime
//     endpoint boundary used by resources.
//   - resources, session, workers, app, testingtools — high-level
//     packages with no v1 imports.
//
// # Core Features
//
//   - **Typed endpoints** — [handlers.Endpoint[Req, Resp]] wraps
//     [endpoint.Endpoint[Req, Resp]] with a canonical handler
//     signature: func(*request.Context, Req) (Resp, error).
//
//   - **Transport-aware routing** — [routing.Handler] receives
//     (*request.Context, routing.Transport), separating request state
//     from response writing.
//
//   - **RFC 9457 Problems** — [response.Problem] and
//     [response.MapperRegistry] provide structured error responses
//     with a frozen, immutable registry.
//
//   - **Telemetry via slog** — [telemetry.Sink] and
//     [telemetry.SlogSink] replace v1's webFramework.AddLog for the
//     Splunk transaction pipeline.
//
//   - **Framework-agnostic** — [routing.Router] and
//     [routing.RouteGroup] work across Gin, Fiber, chi, and net/http
//     via adapter packages.
//
//   - **Pluggable renderers** — [renderers.Renderer] interface with
//     built-in JSON, XML, text, and CSV renderers.
//
//   - **Sessions** — [session.Manager] with cookie store, flash
//     messages, and typed session access via [session.FromContext].
//
//   - **Workers and scheduler** — [workers.InProcessWorker] and
//     [workers.Scheduler] with telemetry-based observability.
//
//   - **Application bootstrap** — [app.Bootstrap] composes the
//     executor, router, worker pool, scheduler, and session manager
//     with Problem-based error handling.
//
// # Migration from v1
//
// See MIGRATION.md for a detailed migration guide. The v1 module
// (github.com/hmmftg/requestCore) remains supported and stable.
package requestcore
