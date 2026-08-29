# requestCore v2

[![Go Reference](https://pkg.go.dev/badge/github.com/hmmftg/requestCore/v2.svg)](https://pkg.go.dev/github.com/hmmftg/requestCore/v2)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hmmftg/requestCore?filename=v2/go.mod)](https://go.dev/dl/)

A **generics-first**, framework-agnostic HTTP application toolkit for Go.
Requires **Go 1.27+**.

> **Status:** v2 has **no released tags** and is under active redesign.
> The current API is an unreleased alpha and will change before the first
> stable v2 release. See [MIGRATION.md](MIGRATION.md) for the redesign plan
> and migration guidance. v1 (the root module) remains supported and stable.

v2 builds on the root [requestCore](../README.md) module with fully typed
endpoints, resources, sessions, and response helpers — eliminating runtime
type assertions and reflection throughout the request lifecycle.

---

## Why v2?

v1 normalizes request parsing across Gin, Fiber, and net/http but uses
`any` for request/response types, requiring runtime type assertions and
reflection. v2 leverages Go generics to make the entire handler lifecycle
**compile-time type-safe**:

- `Endpoint[Req, Resp]` flows types through parse → initialize → handle → render → finalize
- `Resource[ID cmp.Ordered]` constrains resource IDs to comparable types
- `GetTyped[T]` / `SetTyped[T]` eliminate session value type assertions
- `OKTyped[Resp]` renders typed responses without `any`

---

## Features

- **Generic typed endpoints** — `handlers.Endpoint[Req, Resp]` with typed lifecycle hooks
- **Generic resources** — `resources.Resource[ID cmp.Ordered]` with 7 CRUD operations, registered via `ResourceBuilder[ID]` fluent API
- **TypedResource** — advanced 14-type-parameter interface (overkill for simple CRUD; use only when strictest per-operation type guarantees are needed)
- **Typed session access** — `GetTyped[T]` / `SetTyped[T]` generic accessors
- **Generic response helpers** — `OKTyped[Resp]` / `OKWithStatusTyped[Resp]`
- **Framework-agnostic routing** — Gin, Fiber, chi, net/http via adapters
- **Pluggable renderers** — JSON, XML, text, CSV
- **Error handler registry** — per-status handlers with legacy fallback
- **Background workers** — bounded pool with retry, tracing, and mandatory observability
- **Scheduler** — periodic background tasks
- **Sessions & flash** — cookie store with signed tokens
- **CLI** — `requestcore` code generator for handlers, resources, middleware, projects

---

## Quick Start

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "os/signal"
    "syscall"

    "github.com/hmmftg/requestCore/libRequest"
    "github.com/hmmftg/requestCore/webFramework"

    "github.com/hmmftg/requestCore/v2/app"
    "github.com/hmmftg/requestCore/v2/handlers"
    "github.com/hmmftg/requestCore/v2/renderers"
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

    // Register a typed GET endpoint
    err = handlers.GetEndpoint[struct{}, HealthResp](
        application.Router, nil, application.RespHandler, "/health",
        func(req *struct{}, trx *handlers.HandlerRequest[struct{}, HealthResp]) (HealthResp, error) {
            webFramework.AddLog(trx.W, "health-check", slog.String("status", "healthy"))
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

### Generic Endpoints

`handlers.Endpoint[Req, Resp]` is the typed descriptor for a route handler.
The `Req` and `Resp` type parameters flow through the entire lifecycle
without type erasure:

```go
e := handlers.NewEndpoint[MyReq, MyResp]("my-handler", libRequest.JSON, MyHandler).
    WithPath("/users").
    WithInitializer(func(trx *handlers.HandlerRequest[MyReq, MyResp]) error {
        // Runs after parsing, before the handler. Compile-time typed.
        return nil
    }).
    WithFinalizer(func(trx *handlers.HandlerRequest[MyReq, MyResp]) {
        // Always runs, even on panic. Compile-time typed.
    }).
    WithPersistence(persister) // typed persister

handlers.RegisterEndpoint(router, core, respHandler, "POST", "/users", e)
```

Lifecycle hooks are **methods** on `*Endpoint[Req, Resp]`, not free
functions — this means type mismatches are caught at compile time, not
at runtime via reflection.

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

**Recommended path for v2 migration:** implement `Resource[ID]` and
register via `ResourceBuilder`. Each operation returns
`handlers.EndpointRuntime` (satisfied by `*handlers.Endpoint[Req, Resp]`).
Operations returning nil are skipped or registered with a 405 default
handler.

```go
type UserResource struct{}

func (r *UserResource) List() handlers.EndpointRuntime {
    return handlers.NewEndpoint[struct{}, UserListResp](
        "list-users", libRequest.NoBinding,
        func(req *struct{}, trx *handlers.HandlerRequest[struct{}, UserListResp]) (UserListResp, error) {
            return UserListResp{Users: []User{}}, nil
        },
    )
}
// ... similarly for Show, New, Create, Edit, Update, Destroy

// Register via ResourceBuilder (recommended)
err := resources.NewResource[string]("/users").
    EnablePatch().
    WithCustom(reloadOp).
    Register(application.Router, core, application.RespHandler, &UserResource{})
```

**Advanced: TypedResource (14 type parameters).** For cases requiring the
strictest compile-time guarantees on every operation's request/response
types simultaneously, implement `TypedResource[ID, ListReq, ListResp, ...]`.
This is overkill for simple CRUD + custom resources — the 14 type
parameters add verbosity without practical benefit for most use cases.
Any `TypedResource` automatically satisfies `Resource[ID]`.

### Typed Session Access

Session values are stored as `any` but accessed via generic functions
for compile-time type safety:

```go
// Store a typed value
session.SetTyped(sess, "user_id", 42)

// Retrieve with compile-time type checking
userID, err := session.GetTyped[int](sess, "user_id")
```

The `RequestContext.Session` field is typed as `webFramework.SessionContext`
(an interface), not `any` — you can call `Get`, `Set`, `Delete` directly
without type-asserting to `*session.Session`.

### Generic Response Helpers

```go
// Typed response (compile-time type safety)
respHandler.OKTyped(req, MyResp{ID: "1"})

// Typed response with custom status
respHandler.OKWithStatusTyped(req, http.StatusCreated, MyResp{ID: "1"})
```

### Framework Adapters

Switch frameworks by changing one config field:

```go
app.Bootstrap(app.Config{Framework: app.FrameworkGin})    // Gin
app.Bootstrap(app.Config{Framework: app.FrameworkFiber})  // Fiber
app.Bootstrap(app.Config{Framework: app.FrameworkChi})    // chi + net/http
```

All handlers, resources, and middleware work unchanged across frameworks.

### Background Workers

```go
err := application.Worker.Submit(context.Background(), workers.Job{
    Name: "send-email",
    Handler: func(ctx *workers.JobContext) error {
        webFramework.AddLog(ctx.WebFramework, webFramework.HandlerLogTag,
            slog.String("recipient", email))
        // ... send email ...
        return nil
    },
    Options: workers.JobOptions{MaxAttempts: 3},
})
```

For periodic tasks, use `application.Scheduler.Schedule(...)`.

### Renderers

```go
app.Bootstrap(app.Config{
    Framework: app.FrameworkChi,
    Renderer:  renderers.JSONRenderer{},  // or XMLRenderer{}, TextRenderer{}, CSVRenderer{}
})
```

---

## Package Map

| Package | Description |
|---------|-------------|
| `app` | Bootstrap, Config, App — application entry point |
| `handlers` | `Endpoint[Req, Resp]`, `HandlerRequest`, lifecycle hooks, `RegisterEndpoint` |
| `resources` | `Resource[ID]`, `TypedResource`, `ResourceBuilder[ID]`, `Register`, `CustomOperation` |
| `routing` | `Router`, `RouteGroup`, `Middleware`, `Chain` — framework-agnostic routing |
| `session` | `Session`, `Flash`, `Manager`, `CookieStore`, `GetTyped[T]`, `SetTyped[T]` |
| `workers` | `InProcessWorker`, `Scheduler`, `Job`, `JobContext` |
| `renderers` | `Renderer` interface, JSON/XML/text/CSV renderers |
| `response` | `Handler`, `Registry`, `OKTyped[Resp]`, `OKWithStatusTyped[Resp]` |
| `webFramework` | `RequestContext`, `RequestParser`, `SessionContext`, `FlashContext` |
| `libGin` | Gin adapter |
| `libFiber` | Fiber adapter |
| `libChi` | chi adapter |
| `libNetHttp` | net/http adapter |
| `testingtools` | Test helpers and mock infrastructure |
| `cmd/requestcore` | CLI for code generation |

---

## CLI

```bash
# Build the CLI
go build -o requestcore ./cmd/requestcore/cmd

# Generate a handler
./requestcore generate handler user-profile

# Generate a resource (7 CRUD operations)
./requestcore generate resource user

# Generate middleware
./requestcore generate middleware auth

# Generate a new project
./requestcore generate project my-app
```

---

## Observability

`webFramework.AddLog` is **mandatory** for all external API calls,
transaction steps, and critical business events. It flows into the
Splunk transaction pipeline. Never replace it with `slog.*` or `log.*`.

The handler lifecycle automatically emits:
- `<title>-req` (success) — contains the parsed response. If the response
  implements `slog.LogValuer`, the masked projection is logged instead of
  the raw response. The returned HTTP response is never modified.
- `<title>-req-failed` (failure) — contains the error.

Session load failures emit `session-load-failed` (without the raw cookie
token). Session save failures emit `session-save-failed`.

---

## Examples

- [examples/simple/](examples/simple/) — chi-based example with typed endpoints, CRUD resource, workers, and sessions

---

## Migration from v1

See [MIGRATION.md](MIGRATION.md) for the complete v1-to-v2 migration guide.

---

## License

See [LICENSE](../LICENSE) for license information.
