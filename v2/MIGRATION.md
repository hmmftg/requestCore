# Migrating from v1 to v2

This guide covers migrating existing `requestCore` v1 applications to the
v2 module (`github.com/hmmftg/requestCore/v2`) using the **canonical
kernel API**.

> **Status:** v2 has **no released tags** and is under active development.
> The canonical kernel API described here is the basis for the first stable
> v2 release. v1 (the root module) remains supported and stable. See
> [Deferred Tranche 5 Features](#deferred-tranche-5-features) for
> capabilities not yet ported to the v2 kernel.

## Overview

v2 is a **separate Go module** that lives in the `v2/` directory. It
introduces a canonical, stdlib-first kernel that is framework-neutral and
does not import `webFramework`. v1 code continues to work unchanged; v2
can coexist with v1 in the same project during migration.

The v2 kernel replaces the alpha API (typed lifecycle hooks, `HandlerRequest`,
`OKTyped`, `webFramework.AddLog` in v2 context, etc.) with a canonical
handler signature, an `endpoint.Executor` lifecycle, RFC 9457 problem
responses, and `telemetry.Sink` observability.

## Key Changes in v2

| Feature | v1 | v2 (canonical) |
|---------|----|----|
| Module path | `github.com/hmmftg/requestCore` | `github.com/hmmftg/requestCore/v2` |
| Go version | 1.27.0 | **1.27.0** |
| Framework coupling | Gin-specific (`libContext.InitContext`) | Framework-agnostic (`request.Context` + `routing.Transport`) |
| Handler signature | `func(context.Context)` writes via v1 responder | `func(ctx *request.Context, req Req) (Resp, error)` |
| Routing | Framework-specific | `routing.Router` / `routing.RouteGroup` with canonical `{id}` syntax |
| Handler descriptor | `BaseHandler` returns `any` | `handlers.Endpoint[Req, Resp]` wrapping `endpoint.Endpoint[Req, Resp]` |
| Lifecycle | Manual / v1 hooks | `endpoint.Executor`: bind → validate → execute → encode → commit → observe |
| Lifecycle hooks | N/A | **Deferred to Tranche 5** (initializers, finalizers, persistence) |
| Response writing | `response.WebHanlder` | `routing.Transport` + `endpoint.Executor` commit; renderer encoders |
| Errors | `ErrorResponse` / status resolver | `response.Problem` (RFC 9457) via `response.MapperRegistry` |
| Response helpers | `OK(any)` | Handler returns `(Resp, error)`; executor encodes and commits |
| Observability | `webFramework.AddLog` (Splunk pipeline) | `telemetry.Sink` / `telemetry.SlogSink` (slog → Splunk) |
| Resources | Manual route registration | `ResourceBuilder[ID]` + `Resource[ID cmp.Ordered]` |
| Session access | N/A | `session.FromContext` + `GetTyped[T]` / `SetTyped[T]` |
| Sessions | Not built-in | `session.Manager` with `CookieStore` |
| Workers | Not built-in | `workers.InProcessWorker` with retry + `telemetry.Sink` |
| Operation metadata | N/A | `operation.Operation` + `operation.Registry` (frozen at startup) |

## Breaking Changes from the Alpha API

If you previously used the v2 alpha API, the canonical kernel introduces
these breaking changes:

- **`HandlerRequest[Req, Resp]` removed.** Handlers no longer receive a
  `*HandlerRequest`. The canonical signature is
  `func(ctx *request.Context, req Req) (Resp, error)`.
- **`WithInitializer` / `WithFinalizer` / `WithPersistence` removed.**
  Typed lifecycle hooks are deferred to Tranche 5. Use the
  `endpoint.Executor` lifecycle and before-commit hooks on
  `request.Context` (`ctx.AddBeforeCommitHook`) for pre-commit work.
- **`OKTyped[Resp]` / `OKWithStatusTyped[Resp]` removed.** Handlers return
  `(Resp, error)` directly; the executor encodes and commits the response.
  Set a custom success status via `endpoint.WithSuccessStatus`.
- **`webFramework.AddLog` not used in v2.** The v2 kernel is stdlib-only.
  Observability flows through `telemetry.Sink` (default
  `telemetry.SlogSink`). Do not call `webFramework.AddLog` from v2
  handler/worker code.
- **`handlers.NewEndpoint` replaced by `handlers.New`.** The canonical
  constructor is `handlers.New[Req, Resp](opID, method, pattern, handler)`
  or the convenience constructors `handlers.Get`/`Post`/`Put`/`Patch`/
  `Delete`/`Head`.
- **`RegisterEndpoint` signature changed.** It now takes
  `(router, exec *endpoint.Executor, ep *Endpoint[Req, Resp])` — no
  `core` or `respHandler` arguments.
- **`app.Bootstrap` config changed.** `LegacyCore` / `LegacyHandler` are
  gone. The `App` exposes `Executor`, `ProblemMapper`, and
  `OperationRegistry` instead of `RespHandler` / `Core`.
- **`TypedResource` is not part of the recommended canonical API.** Use
  `Resource[ID]` + `ResourceBuilder` for all CRUD resources.
- **`resources.GetParsedID` takes `*request.Context`** (not
  `*HandlerRequest`): `resources.GetParsedID[string](ctx, "id")`.
- **Route patterns use canonical `{id}` syntax.** Adapters translate to
  framework-specific syntax automatically.

## Step 1: Add v2 as a dependency

The v2 module is released via a manual, prerelease-aware GitHub Actions
workflow (`release-v2.yml`). The workflow creates tags in the format
`v2/v2.0.0-alpha.0` (subdirectory module format). Go strips the `v2/`
prefix when resolving, so the consumer command uses the version portion
without the `v2/` prefix.

Until the first stable v2 tag is published, pin to the latest prerelease
tag:

```bash
cd your-project
# Replace v2.0.0-alpha.0 with the latest published v2 prerelease tag.
# Check available tags at:
#   https://github.com/hmmftg/requestCore/releases
go get github.com/hmmftg/requestCore/v2@v2.0.0-alpha.0
```

> **Note:** `go get ...@latest` will not resolve until the first
> non-prerelease v2 tag is published. Always pin to a specific tag.
> If no tags have been published yet, the release workflow must be
> triggered manually from the GitHub Actions tab.

## Step 2: Bootstrap the v2 App

Replace your manual framework setup with `app.Bootstrap`. The canonical
`App` composes the router, `endpoint.Executor`, `response.MapperRegistry`,
`operation.Registry`, worker pool, scheduler, and session manager:

```go
import "github.com/hmmftg/requestCore/v2/app"

application, err := app.Bootstrap(app.Config{
    Framework: app.FrameworkChi, // or FrameworkGin, FrameworkFiber, FrameworkNetHTTP
    Renderer:  renderers.JSONRenderer{},
})
if err != nil {
    log.Fatal(err)
}
defer application.Close()
```

The `App` exposes:

| Field | Type | Purpose |
|-------|------|---------|
| `Router` | `routing.Router` | Route registration |
| `Executor` | `*endpoint.Executor` | Canonical lifecycle runner |
| `ProblemMapper` | `*response.MapperRegistry` | Error-to-Problem mapping |
| `OperationRegistry` | `operation.Registry` | Operation metadata (frozen at start) |
| `Worker` | `workers.Worker` | Background job pool |
| `Scheduler` | `*workers.Scheduler` | Periodic tasks |
| `Sessions` | `*session.Manager` | Session management |

Customize the executor, problem mapper, operation registry, telemetry
sink, global middleware, and not-found / method-not-allowed handlers via
`app.Config` fields. The operation registry and problem mapper are frozen
at `StartWithContext` so routes and mappings can be added between
`Bootstrap` and `Start`.

## Step 3: Migrate Handlers

### v1 Handler (Gin-specific)

```go
func MyHandler(c context.Context) {
    w := libContext.InitContext(c)
    // ... handler logic
    core.Responder().OK(w, resp)
}

engine.GET("/users", libGin.Gin(MyHandler))
```

### v2 Handler (Canonical, typed)

v2 handlers use the canonical signature
`func(ctx *request.Context, req Req) (Resp, error)`. The executor handles
binding, validation, encoding, and committing the response.

```go
import (
    "github.com/hmmftg/requestCore/v2/handlers"
    "github.com/hmmftg/requestCore/v2/request"
)

type CreateUserReq struct {
    Name string `json:"name"`
}
type CreateUserResp struct {
    ID string `json:"id"`
}

// Register as a POST endpoint with JSON body binding (one-call form):
err := handlers.PostEndpoint[CreateUserReq, CreateUserResp](
    application.Router, application.Executor, "/users",
    func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
        return CreateUserResp{ID: "1"}, nil
    },
)

// Or create then register (useful when you need to configure the endpoint):
ep := handlers.Post[CreateUserReq, CreateUserResp](
    "create-user", "/users",
    func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
        return CreateUserResp{ID: "1"}, nil
    },
)
err = handlers.RegisterEndpoint(application.Router, application.Executor, ep)
```

### Advanced Endpoint Configuration

Access the wrapped `endpoint.Endpoint` via `Inner()` to apply
`endpoint.Option` functions (validator, success status, encoder, declared
problems, tags, deprecation):

```go
ep := handlers.Post[CreateUserReq, CreateUserResp]("create-user", "/users", handler)
ep.Inner().
    WithSuccessStatus(http.StatusCreated).
    WithTags("users", "write")
```

### Returning Errors

Handlers return `(Resp, error)`. Returning a non-nil error routes it
through the `response.MapperRegistry`, which maps it to an RFC 9457
`response.Problem` and writes it through the transport. Return a
`*response.Problem` directly for full control:

```go
func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
    if exists(req.Name) {
        return CreateUserResp{}, response.NewProblemWithCode(
            http.StatusConflict, "Conflict", "USER_EXISTS",
        ).WithDetail("a user with that name already exists")
    }
    return CreateUserResp{ID: "1"}, nil
}
```

## Step 4: Migrate to Resources

For CRUD endpoints, implement `resources.Resource[ID]` (each operation
returns `handlers.EndpointRuntime`) and register via `ResourceBuilder`:

```go
import (
    "github.com/hmmftg/requestCore/v2/handlers"
    "github.com/hmmftg/requestCore/v2/request"
    "github.com/hmmftg/requestCore/v2/resources"
)

type ItemResource struct{}

func (r *ItemResource) List() handlers.EndpointRuntime {
    return handlers.Get[struct{}, []ItemResp]("list-items", "/items",
        func(ctx *request.Context, req struct{}) ([]ItemResp, error) {
            return []ItemResp{}, nil
        },
    )
}

func (r *ItemResource) Show() handlers.EndpointRuntime {
    return handlers.Get[struct{}, ItemResp]("show-item", "/items/{id}",
        func(ctx *request.Context, req struct{}) (ItemResp, error) {
            id, err := resources.GetParsedID[string](ctx, "id")
            if err != nil {
                return ItemResp{}, err
            }
            return ItemResp{ID: id}, nil
        },
    )
}
// ... similarly for New, Create, Edit, Update, Destroy

// Register via ResourceBuilder (recommended).
err := resources.NewResource[string]("/items").
    EnablePatch().
    Register(application.Router, application.Executor, &ItemResource{})
```

`EnablePatch()` registers PATCH as an alias for Update. Operations
returning nil are skipped, or registered with a default 405 handler when
`WithDefaults` is set. Use `WithIDParam` / `WithIDParser` for non-default
ID parameter names or non-string ID types.

### Custom Operations (non-CRUD actions)

For non-standard actions like `Reload`, use `WithCustom`:

```go
reloadOp := resources.CustomOperation{
    Method: "POST",
    Path:   "/reload",
    Endpoint: handlers.Post[struct{}, ReloadResp](
        "reload-parameters", "/parameters/reload",
        func(ctx *request.Context, req struct{}) (ReloadResp, error) {
            return ReloadResp{Reloaded: true}, nil
        },
    ),
}

err := resources.NewResource[string]("/parameters").
    WithCustom(reloadOp).
    Register(application.Router, application.Executor, &ParameterResource{})
```

Custom operations are registered before `/{id}` routes to ensure correct
precedence (e.g. `POST /parameters/reload` won't match `/{id}`).

## Step 5: Switch Frameworks

Change the `Framework` field — all handlers, resources, and middleware
work unchanged:

```go
app.Bootstrap(app.Config{Framework: app.FrameworkGin})     // Gin
app.Bootstrap(app.Config{Framework: app.FrameworkFiber})   // Fiber
app.Bootstrap(app.Config{Framework: app.FrameworkChi})     // chi + net/http
app.Bootstrap(app.Config{Framework: app.FrameworkNetHTTP}) // net/http
```

Route patterns use canonical `{id}` syntax; adapters translate to the
framework-specific form (`:id` for Gin/Fiber, `{id}` for chi).

## Step 6: Add Sessions

```go
import "github.com/hmmftg/requestCore/v2/session"

store, err := session.NewCookieStore(session.CookieStoreConfig{
    SecretKey: []byte("your-32-byte-secret-key-here-32-bytes"),
})
if err != nil {
    log.Fatal(err)
}

application, err := app.Bootstrap(app.Config{
    Framework:      app.FrameworkChi,
    SessionStore:   store,
    SessionSecret:  "your-signing-secret",
})
if err != nil {
    log.Fatal(err)
}

// Add session middleware to a route group.
api := application.Register("/api",
    app.SessionMiddleware(application.Sessions, "session"),
)
```

For configurable session save failure handling, use
`session.MiddlewareWithConfig` with `SaveStrict` (default, propagates save
failures) or `SaveBestEffort` (logs via the telemetry sink but does not
propagate):

```go
session.MiddlewareWithConfig(session.MiddlewareConfig{
    Manager:         application.Sessions,
    CookieName:      "session",
    SaveFailureMode: session.SaveBestEffort,
})
```

### Typed Session Access

The session is stored on the `request.Context` via a typed key. Retrieve
it with `session.FromContext` and use the generic accessors for
compile-time type safety:

```go
func(ctx *request.Context, req struct{}) (ProfileResp, error) {
    sess := session.FromContext(ctx)
    if sess == nil {
        return ProfileResp{}, errors.New("no session")
    }

    session.SetTyped(sess, "visits", 1)

    visits, err := session.GetTyped[int](sess, "visits")
    if err != nil {
        visits = 0
    }
    return ProfileResp{Visits: visits}, nil
}
```

## Step 7: Add Background Workers

Workers run outside HTTP request contexts but have full `telemetry.Sink`
observability. Each job receives a `*workers.JobContext` with a job-scoped
`*slog.Logger` and a `telemetry.Sink`:

```go
import (
    "log/slog"
    "github.com/hmmftg/requestCore/v2/workers"
)

err := application.Worker.Submit(context.Background(), workers.Job{
    Name: "send-email",
    Handler: func(ctx *workers.JobContext) error {
        // ctx.Logger is a job-scoped *slog.Logger (TransactionSink-backed).
        // ctx.Sink is the telemetry.Sink for lifecycle events.
        ctx.Logger.Info("sending email", slog.String("recipient", email))
        // ... send email ...
        return nil
    },
    Options: workers.JobOptions{
        MaxAttempts:    3,
        InitialBackoff: 1 * time.Second,
    },
})
```

The worker pool emits `worker-<name>-req` (success) and
`worker-<name>-req-failed` (failure) telemetry events after each attempt.

## Step 8: Periodic Background Tasks (Scheduler)

For long-running poller loops, use `application.Scheduler` with a
`workers.ScheduledJob`. Each tick receives a fresh `*workers.JobContext`
with the same telemetry observability:

```go
err := application.Scheduler.Schedule(workers.ScheduledJob{
    Name:     "data-sync",
    Interval: 60 * time.Second,
    Handler: func(ctx *workers.JobContext) error {
        ctx.Logger.Info("syncing data")
        // ... poll and process ...
        return nil
    },
})
```

The scheduler is started by `StartWithContext` and shut down automatically
by `app.Shutdown` alongside the HTTP server and worker pool.

## Observability: telemetry.Sink

The v2 kernel records request and transaction lifecycle events through
`telemetry.Sink` (default `telemetry.SlogSink`), not `webFramework.AddLog`.
The production slog handler ingests these records into Splunk, preserving
the canonical `<operation>-req` / `<operation>-req-failed` outcome keys.

The executor automatically emits start, success, and failure events for
every request. Both success and failure paths are recorded: success events
include the safely projected response (never raw bodies); failure events
include the error. Request and response bodies, credentials, cookies, and
secrets are never included in telemetry attributes.

v2 packages must record lifecycle events through `telemetry.Sink`. Direct
`slog.*` / `log.*` calls are only permitted for startup/diagnostic
messages, not transaction tracing. `telemetry.NopSink` is acceptable in
tests only — production executor/app construction defaults to an
observable `SlogSink`.

Configure a custom sink via `app.Config.TelemetrySink` or
`endpoint.WithTelemetrySink`. v1 projects continue to use
`webFramework.AddLog`; v2 projects use `telemetry.SlogSink` as the
observability path.

## Coexistence with v1

v1 and v2 can coexist in the same project. The `go.work` file manages both
modules:

```bash
# From the requestCore repo root
go work use . v2
```

This allows importing both v1 and v2 packages simultaneously during
migration. When using `-mod=vendor`, both modules coexist in the
`vendor/` directory; run `go mod vendor` after adding both dependencies.

## Graceful Shutdown

Use `app.Shutdown` for coordinated HTTP + worker + scheduler shutdown:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := application.Shutdown(ctx); err != nil {
    log.Fatal(err)
}
```

`Shutdown` stops the HTTP server, then the worker pool, then the
scheduler, returning the first non-nil error. `http.ErrServerClosed` is
treated as a clean termination.

## Deferred Tranche 5 Features

The following capabilities are **not yet** part of the canonical v2 kernel
and are deferred to Tranche 5. Until then, use v1 for these concerns or
implement them in application code:

- **Persistence** — typed persisters and the v1 `request` table audit
  trail are not yet ported. The v2 executor lifecycle does not include a
  persistence hook.
- **Tracing** — OpenTelemetry trace propagation/integration is not yet
  wired into the v2 executor. The `request.Context` carries `TraceID` but
  no span management is provided.
- **Initializers** — the `WithInitializer` typed pre-handler hook from
  the alpha API is removed. Use `ctx.AddBeforeCommitHook` for pre-commit
  work, or perform initialization inside the handler function.
- **Finalizers** — the `WithFinalizer` typed always-run hook from the
  alpha API is removed. Use `defer` inside the handler function, or wait
  for the Tranche 5 finalizer hook.
- **Recovery callbacks** — panic recovery with custom callbacks is not
  yet exposed on the v2 executor. The scheduler recovers panics per tick;
  the executor path will gain configurable recovery in Tranche 5.
- **ID parsers** — `resources.ResourceBuilder.WithIDParser` exists for
  custom ID parsing, but richer ID parser infrastructure (registry,
  shared parsers) is deferred to Tranche 5.

## FAQ / Troubleshooting

### Go version requirement

Both v1 and v2 require **Go 1.27.0**. v2 uses generic methods and
type-parameterized constructors (`handlers.New[Req, Resp]`,
`endpoint.Endpoint[Req, Resp]`) which require Go 1.27+.

### API call infrastructure (v1 issues.md capabilities)

The 9 capabilities described in the v1 `issues.md` file (base header
builder, instrumented HTTP client, standardized API call logging, status
code extraction, error normalization, timeout guards, retry framework,
URL tracker-ID extraction, skip-pattern matching) are implemented in the
v1 module:

- `handlers.BuildBaseRemoteHeaders` — base header builder
- `libCallApi.NewInstrumentedHTTPClient` — instrumented HTTP client
- `handlers.LogApiCall` — standardized API call logging
- `handlers.ExtractStatusCode` / `handlers.DeriveStatusCode` — status extraction
- `handlers.NormalizeCallError` / `handlers.BuildTimeoutError` — error normalization
- `libRetry` — retry framework with backoff and jitter

These are available to v2 consumers via the v1 import. A v2-native API
call layer is deferred to a later tranche.

### Worker audit-trail limitation

Worker jobs run outside HTTP request contexts and do not automatically
insert rows into the v1 `request` persistence table. Worker observability
is via `telemetry.Sink` (slog → Splunk), not via the persistence audit
trail. If your worker jobs need audit-trail persistence, call your
persistence layer manually inside the job handler. This is a known design
tradeoff that will be revisited in Tranche 5.

### No tags published yet

If `go get github.com/hmmftg/requestCore/v2@v2.0.0-alpha.0` fails with
"unknown revision", no v2 tags have been published yet. The repository
maintainer must trigger the `Release v2` workflow from the GitHub
Actions tab (manual dispatch) to create the first prerelease tag.

## Checklist

- [ ] Add v2 dependency (pin to a specific prerelease tag)
- [ ] Bootstrap v2 App with `app.Bootstrap` (no legacy core/handler)
- [ ] Migrate handlers to the canonical signature `func(ctx *request.Context, req Req) (Resp, error)`
- [ ] Register routes via `handlers.RegisterEndpoint` / `*Endpoint` constructors
- [ ] Migrate CRUD endpoints to `ResourceBuilder` + `Resource[ID]`
- [ ] Add custom operations for non-CRUD actions (e.g. Reload) via `WithCustom`
- [ ] Map domain errors to `response.Problem` via `response.MapperRegistry`
- [ ] Add session middleware (`app.SessionMiddleware`) if needed
- [ ] Use `session.FromContext` + `GetTyped[T]` / `SetTyped[T]` for session access
- [ ] Add background workers (`application.Worker.Submit`) and schedulers
- [ ] Verify `telemetry.Sink` is configured (default `SlogSink`); do not use `webFramework.AddLog` in v2 code
- [ ] Use `app.Shutdown` for coordinated HTTP + worker + scheduler shutdown
- [ ] Run cross-framework conformance tests
- [ ] Update CI to test v2 module
- [ ] Verify vendor mode coexistence if using `-mod=vendor`
