# requestCore v2

[![Go Reference](https://pkg.go.dev/badge/github.com/hmmftg/requestCore/v2.svg)](https://pkg.go.dev/github.com/hmmftg/requestCore/v2)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hmmftg/requestCore?filename=v2/go.mod)](https://go.dev/dl/)

A **generics-first**, framework-agnostic HTTP application toolkit for Go.
Requires **Go 1.27+**.

> **Status:** v2 has **no released tags** and is under active development.
> The API described here is the canonical kernel API and will remain the
> basis for the first stable v2 release, but minor refinements may still
> occur before a tag is cut. See [MIGRATION.md](MIGRATION.md) for the
> migration guide and the Tranche 5 lifecycle features (persistence,
> tracing, initializers, finalizers, recovery callbacks, ID parsers).
> v1 (the root module) remains supported and stable.

v2 builds on the root [requestCore](../README.md) module with a canonical,
stdlib-first kernel: typed endpoints, a framework-neutral routing contract,
RFC 9457 problem responses, structured telemetry via `slog`, and typed
session access — all without runtime type assertions or reflection in the
request lifecycle.

---

## Why v2?

v1 normalizes request parsing across Gin, Fiber, and net/http but couples
handlers to the v1 `webFramework` stack and uses `any` for request/response
types, requiring runtime type assertions. v2 introduces a **canonical
kernel** that is framework-neutral and stdlib-only:

- **Canonical handler signature** — `func(ctx *request.Context, req Req) (Resp, error)` flows types through bind → validate → execute → encode → commit → observe without type erasure.
- **Framework-neutral routing** — `routing.Handler` is `func(*request.Context, routing.Transport) error`; Gin, Fiber, chi, and net/http adapters all translate the same canonical `{id}` pattern syntax.
- **RFC 9457 errors** — every error is mapped to a `response.Problem` via a `response.MapperRegistry`, never serialized raw.
- **Structured telemetry** — `telemetry.Sink` (default `telemetry.SlogSink`) records lifecycle events through `slog`, ingested by Splunk. The v2 kernel does not import `webFramework`.
- **Typed sessions** — `session.FromContext` retrieves a `*Session` stored via `request.Context` typed keys; `GetTyped[T]`/`SetTyped[T]` give compile-time type safety.

---

## Features

- **Typed endpoints** — `handlers.Endpoint[Req, Resp]` wraps `endpoint.Endpoint[Req, Resp]` with operation metadata; convenience constructors `handlers.Get`/`Post`/`Put`/`Patch`/`Delete`/`Head`.
- **Canonical executor** — `endpoint.Executor` runs the full lifecycle (bind → validate → execute → encode → commit → observe) with telemetry and problem mapping.
- **Resources** — `resources.Resource[ID cmp.Ordered]` with 7 CRUD operations, registered via the `ResourceBuilder[ID]` fluent API.
- **Typed session access** — `session.FromContext`, `GetTyped[T]` / `SetTyped[T]` generic accessors.
- **RFC 9457 problem responses** — `response.Problem` and `response.MapperRegistry` for structured error mapping.
- **Framework-agnostic routing** — Gin, Fiber, chi, net/http via adapters; canonical `{id}` path-parameter syntax.
- **Pluggable renderers** — JSON, XML, text, CSV.
- **Structured telemetry** — `telemetry.Sink` / `telemetry.SlogSink` for request and worker lifecycle events.
- **Background workers** — bounded pool with retry and `telemetry.Sink` observability.
- **Scheduler** — periodic background tasks with the same telemetry path.
- **Sessions & flash** — cookie store with signed tokens.

---

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os/signal"
    "syscall"

    "github.com/hmmftg/requestCore/v2/app"
    "github.com/hmmftg/requestCore/v2/handlers"
    "github.com/hmmftg/requestCore/v2/renderers"
    "github.com/hmmftg/requestCore/v2/request"
)

type HealthResp struct {
    Status string `json:"status"`
}

func main() {
    application, err := app.Bootstrap(app.Config{
        Framework: app.FrameworkChi,
        Renderer:  renderers.JSONRenderer{},
    })
    if err != nil {
        log.Fatal(err)
    }
    defer application.Close()

    // Register a typed GET endpoint using the canonical handler signature.
    err = handlers.GetEndpoint[struct{}, HealthResp](
        application.Router, application.Executor, "/health",
        func(ctx *request.Context, req struct{}) (HealthResp, error) {
            return HealthResp{Status: "healthy"}, nil
        },
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    if err := application.StartWithContext(ctx, ":8080"); err != nil {
        log.Fatal(err)
    }
}
```

---

## Core Concepts

### Typed Endpoints

`handlers.Endpoint[Req, Resp]` is the typed descriptor for a route handler.
It wraps `endpoint.Endpoint[Req, Resp]` with operation metadata (operation
ID, HTTP method, route pattern) and satisfies the type-erased
`handlers.EndpointRuntime` interface so resources can return heterogeneous
typed endpoints without exposing type parameters at the registration
boundary.

The canonical v2 handler signature is:

```go
func(ctx *request.Context, req Req) (Resp, error)
```

The `Req` and `Resp` type parameters flow through the entire lifecycle
without type erasure — bind, validate, execute, encode, and commit are all
compile-time type-safe inside `endpoint.Executor`.

```go
// Create a typed endpoint with explicit operation metadata.
ep := handlers.Post[CreateUserReq, CreateUserResp](
    "create-user", "/users",
    func(ctx *request.Context, req CreateUserReq) (CreateUserResp, error) {
        return CreateUserResp{ID: "1", Name: req.Name}, nil
    },
)

// Register it on a route group via the executor.
if err := handlers.RegisterEndpoint(application.Router, application.Executor, ep); err != nil {
    log.Fatal(err)
}
```

For advanced configuration (custom validator, success status, encoder,
declared problems, tags, deprecation), access the wrapped
`endpoint.Endpoint` via `ep.Inner()` and apply `endpoint.Option` functions:

```go
ep := handlers.Post[CreateUserReq, CreateUserResp]("create-user", "/users", handler)
ep.Inner().
    WithSuccessStatus(http.StatusCreated).
    WithTags("users", "write")
```

### Convenience Constructors

`handlers.Get`/`Post`/`Put`/`Patch`/`Delete`/`Head` create a typed
`*Endpoint` with the appropriate method and (for body methods) JSON
binding. The `*Endpoint` variants (`GetEndpoint`, `PostEndpoint`, …) create
and register in one call.

### Resources

`resources.Resource[ID cmp.Ordered]` defines 7 CRUD operations:

| Operation | Method | Path |
|-----------|--------|------|
| List | GET | `/{resource}` |
| Show | GET | `/{resource}/{id}` |
| New | GET | `/{resource}/new` |
| Create | POST | `/{resource}` |
| Edit | GET | `/{resource}/{id}/edit` |
| Update | PUT | `/{resource}/{id}` |
| Destroy | DELETE | `/{resource}/{id}` |

Each operation returns `handlers.EndpointRuntime` (satisfied by
`*handlers.Endpoint[Req, Resp]`). Operations returning nil are skipped, or
registered with a default 405 handler when `WithDefaults` is set.

```go
type ItemResource struct{}

func (r *ItemResource) List() handlers.EndpointRuntime {
    return handlers.Get[struct{}, []ItemResp]("list-items", "/items",
        func(ctx *request.Context, req struct{}) ([]ItemResp, error) {
            return []ItemResp{{ID: "1", Name: "Item 1"}}, nil
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
            return ItemResp{ID: id, Name: "Item " + id}, nil
        },
    )
}
// ... similarly for New, Create, Edit, Update, Destroy

// Register via ResourceBuilder (recommended).
err := resources.NewResource[string]("/items").
    EnablePatch().
    Register(application.Router, application.Executor, &ItemResource{})
```

`EnablePatch()` registers PATCH as an alias for Update. Custom non-CRUD
operations (e.g. Reload) can be added via `WithCustom`.

### Typed Session Access

Session middleware stores the `*Session` on the `request.Context` via a
typed key. Handlers retrieve it with `session.FromContext` and use the
generic accessors for compile-time type safety:

```go
sess := session.FromContext(ctx)
if sess == nil {
    return Resp{}, errors.New("no session")
}

// Store a typed value.
session.SetTyped(sess, "user_id", 42)

// Retrieve with compile-time type checking.
userID, err := session.GetTyped[int](sess, "user_id")
```

### RFC 9457 Problem Responses

Errors returned by handlers are mapped to `response.Problem` (RFC 9457)
via the `response.MapperRegistry` configured on the executor and router.
Unknown errors always become sanitized 500 problems; causes are never
serialized. Register custom mappers before `StartWithContext` (the
registry is frozen at startup):

```go
mapper := response.DefaultMapperRegistry()
_ = mapper.Register(
    func(err error) bool {
        var e *MyError
        return errors.As(err, &e)
    },
    func(err error) *response.Problem {
        return response.NewProblemWithCode(409, "Conflict", "MY_ERROR").
            WithDetail("resource already exists")
    },
)

application, err := app.Bootstrap(app.Config{
    Framework:     app.FrameworkChi,
    ProblemMapper: mapper,
})
```

Handlers can also return a `*response.Problem` directly — the mapper
returns it as-is.

### Framework Adapters

Switch frameworks by changing one config field:

```go
app.Bootstrap(app.Config{Framework: app.FrameworkGin})    // Gin
app.Bootstrap(app.Config{Framework: app.FrameworkFiber})  // Fiber
app.Bootstrap(app.Config{Framework: app.FrameworkChi})    // chi + net/http
app.Bootstrap(app.Config{Framework: app.FrameworkNetHTTP}) // net/http
```

All handlers, resources, and middleware work unchanged across frameworks.
Route patterns use canonical `{id}` syntax; adapters translate to the
framework-specific form (`:id` for Gin/Fiber, `{id}` for chi).

### Telemetry

The v2 kernel records lifecycle events through `telemetry.Sink`. The
default sink is `telemetry.SlogSink`, which emits structured `slog`
records ingested by Splunk. The canonical event names are
`<operation>-req` (success) and `<operation>-req-failed` (failure),
matching the v1 `webFramework.AddLog` key convention so Splunk dashboards
remain consistent.

The executor automatically emits start, success, and failure events for
every request. Request and response bodies are never included in telemetry
attributes; `slog.LogValuer` masking is honored.

Configure a custom sink via `app.Config.TelemetrySink` or
`endpoint.WithTelemetrySink`. `telemetry.NopSink` is intended for tests
only — production setups must use an observable sink.

### Background Workers

```go
err := application.Worker.Submit(context.Background(), workers.Job{
    Name: "send-email",
    Handler: func(ctx *workers.JobContext) error {
        // ctx.Logger is a job-scoped *slog.Logger.
        // ctx.Sink is the telemetry.Sink for lifecycle events.
        ctx.Logger.Info("sending email", slog.String("recipient", email))
        // ... send email ...
        return nil
    },
    Options: workers.JobOptions{MaxAttempts: 3},
})
```

For periodic tasks, use `application.Scheduler.Schedule(...)` with a
`workers.ScheduledJob`. Both the worker pool and scheduler emit
`worker-<name>-req` / `worker-<name>-req-failed` telemetry events and are
shut down automatically by `app.Shutdown`.

### Renderers

```go
app.Bootstrap(app.Config{
    Framework: app.FrameworkChi,
    Renderer:  renderers.JSONRenderer{},  // or XMLRenderer{}, TextRenderer{}, CSVRenderer{}
})
```

The configured renderer is used as the executor's default encoder. Per-endpoint
encoders can be set via `endpoint.WithEncoder`.

---

## Package Map

| Package | Description |
|---------|-------------|
| `app` | `Bootstrap`, `Config`, `App` — application entry point |
| `request` | `Context`, `ResponseState`, typed keys — per-request state |
| `routing` | `Router`, `RouteGroup`, `Handler`, `Middleware`, `Chain`, `Transport` |
| `endpoint` | `Endpoint[Req, Resp]`, `Executor`, `Option`s — canonical lifecycle |
| `handlers` | `Endpoint[Req, Resp]`, `EndpointRuntime`, `New`/`Get`/`Post`/…, `RegisterEndpoint` |
| `operation` | `Operation`, `Registry` — endpoint metadata |
| `binding` | `Plan`, binding modes — request binding |
| `validation` | `Validator` — request validation |
| `response` | `Problem` (RFC 9457), `MapperRegistry`, `Handler`, `Registry` |
| `renderers` | `Renderer` interface, JSON/XML/text/CSV renderers |
| `resources` | `Resource[ID]`, `ResourceBuilder[ID]`, `Register`, `CustomOperation`, `GetParsedID` |
| `session` | `Session`, `Flash`, `Manager`, `CookieStore`, `FromContext`, `GetTyped[T]`, `SetTyped[T]` |
| `telemetry` | `Sink`, `SlogSink`, `NopSink`, `Event` — lifecycle observability |
| `workers` | `InProcessWorker`, `Scheduler`, `Job`, `JobContext`, `ScheduledJob` |
| `adapter` | Framework-agnostic endpoint/transport bridging |
| `libGin` | Gin adapter |
| `libFiber` | Fiber adapter |
| `libChi` | chi adapter |
| `libNetHttp` | net/http adapter |
| `testingtools` | Test helpers and mock infrastructure |

---

## Observability

The v2 kernel records request and transaction lifecycle events through
`telemetry.Sink` (default `telemetry.SlogSink`), not `webFramework.AddLog`.
The production slog handler ingests these records into Splunk, preserving
the canonical `<operation>-req` / `<operation>-req-failed` outcome keys.

Both success and failure paths are recorded: success events include the
safely projected response (never raw bodies); failure events include the
error. Session load/save failures emit `session-load-failed` and
`session-save-failed` events via the configured sink.

v2 packages must record lifecycle events through `telemetry.Sink`. Direct
`slog.*` / `log.*` calls are only permitted for startup/diagnostic
messages, not transaction tracing. `telemetry.NopSink` is acceptable in
tests only.

---

## Examples

- [examples/simple/](examples/simple/) — chi-based example with typed endpoints, CRUD resource, workers, sessions, and typed session access.

---

## Migration from v1

See [MIGRATION.md](MIGRATION.md) for the complete v1-to-v2 migration guide,
including breaking changes from the alpha API and the Tranche 5 lifecycle
features (persistence, tracing, initializers, finalizers, recovery callbacks,
ID parsers).

---

## License

See [LICENSE](../LICENSE) for license information.
