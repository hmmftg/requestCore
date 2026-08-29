# Migrating from v1 to v2

This guide covers migrating existing `requestCore` v1 applications to the v2 module (`github.com/hmmftg/requestCore/v2`).

> **Status:** v2 has **no released tags** and is under active redesign.
> The current API is an unreleased alpha. A breaking redesign is in progress
> to make v2 a typed, framework-neutral HTTP toolkit. The migration guide
> below describes the current alpha API; it will be updated as the redesign
> progresses. v1 (the root module) remains supported and stable.

## Overview

v2 is a **separate Go module** that lives in the `v2/` directory. The current
alpha is backward-compatible at the module level — v1 code continues to work
unchanged. The redesign will introduce a breaking canonical API before the
first stable v2 release.

## Key Changes in v2

| Feature | v1 | v2 |
|---------|----|----|
| Module path | `github.com/hmmftg/requestCore` | `github.com/hmmftg/requestCore/v2` |
| Go version | 1.25.5 | **1.27.0** (required for generic methods) |
| Framework coupling | Gin-specific (`libContext.InitContext`) | Framework-agnostic (`RequestContext`) |
| Response writing | `response.WebHanlder` | `v2/response.Handler` with pluggable renderers |
| Routing | Framework-specific | `routing.Router` interface (Gin, Fiber, chi, net/http) |
| Handlers | `BaseHandler` returns `any` | Generic `Endpoint[Req, Resp]` — fully typed lifecycle |
| Lifecycle hooks | N/A (manual) | `WithInitializer`/`WithFinalizer`/`WithPersistence` typed methods |
| Response helpers | `OK(any)` | `OK(any)` + `OKTyped[Resp]` generic method |
| Resources | Manual route registration | `ResourceBuilder[ID]` + `Resource[ID cmp.Ordered]` (TypedResource is advanced, 14 type params) |
| Session access | N/A | `SessionContext` interface + `GetTyped[T]`/`SetTyped[T]` generic accessors |
| Sessions | Not built-in | `session.Manager` with `CookieStore` |
| Workers | Not built-in | `workers.InProcessWorker` with retry |
| CLI | None | `requestcore` CLI for code generation |

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

Replace your manual framework setup with `app.Bootstrap`:

```go
// Before (v1 with Gin)
engine := gin.New()
// ... manual route registration

// After (v2)
import "github.com/hmmftg/requestCore/v2/app"

application, err := app.Bootstrap(app.Config{
    Framework:     app.FrameworkGin, // or FrameworkChi, FrameworkFiber
    LegacyCore:    v1Core,           // your existing v1 RequestCoreInterface
    LegacyHandler: legacyHandler,    // your existing v1 WebHanlder
})
if err != nil {
    log.Fatal(err)
}
defer application.Close()
```

## Step 3: Migrate Handlers

### v1 Handler (Gin-specific)

```go
func MyHandler(c context.Context) {
    w := libContext.InitContext(c)
    // ... handler logic
    core.Responder().OK(w, resp)
}

// Registration
engine.GET("/users", libGin.Gin(MyHandler))
```

### v2 Handler (Framework-agnostic, typed endpoint)

v2 uses typed endpoint functions instead of the v1 struct-based
`HandlerInterface` pattern. Each handler is a function that receives
a typed `*HandlerRequest[Req, Resp]` and returns `(Resp, error)`.

```go
import (
    "github.com/hmmftg/requestCore/libRequest"
    "github.com/hmmftg/requestCore/v2/handlers"
)

// Define your request and response types.
type MyReq struct {
    Name string `json:"name"`
}
type MyResp struct {
    ID string `json:"id"`
}

// MyHandler is the handler function for the endpoint.
func MyHandler(req *MyReq, trx *handlers.HandlerRequest[MyReq, MyResp]) (MyResp, error) {
    // trx.W is the legacy webFramework.WebFramework for AddLog calls.
    // trx.V2 is the v2 RequestContext for direct parser access.
    return MyResp{ID: "1"}, nil
}

// Register as a POST endpoint with JSON body binding:
handlers.PostEndpoint[MyReq, MyResp](
    application.Router, core, application.RespHandler,
    "/users",
    MyHandler,
)

// Or as a GET endpoint with no body binding:
handlers.GetEndpoint[struct{}, MyResp](
    application.Router, core, application.RespHandler,
    "/health",
    func(req *struct{}, trx *handlers.HandlerRequest[struct{}, MyResp]) (MyResp, error) {
        return MyResp{ID: "ok"}, nil
    },
)
```

### Lifecycle Hooks (Initializer, Finalizer)

For handlers that need initialization or finalization, use the typed
`WithInitializer` and `WithFinalizer` **methods** on `*Endpoint[Req, Resp]`.
These are fully typed and compile-time checked — no reflection, no
runtime type-mismatch panics:

```go
e := handlers.NewEndpoint[MyReq, MyResp]("my-handler", libRequest.JSON, MyHandler).
    WithPath("/users").
    WithInitializer(func(trx *handlers.HandlerRequest[MyReq, MyResp]) error {
        // Runs after parsing, before the main handler.
        return nil
    }).
    WithFinalizer(func(trx *handlers.HandlerRequest[MyReq, MyResp]) {
        // Always runs, even on panic. Best-effort.
    })

handlers.RegisterEndpoint(application.Router, core, application.RespHandler, "POST", "/users", e)
```

### Typed Response Helpers

For compile-time type-safe responses, use the generic `OKTyped[Resp]`
and `OKWithStatusTyped[Resp]` methods on `response.Handler`:

```go
// Instead of: respHandler.OK(req, MyResp{ID: "1"})
// Use the typed version:
err := application.RespHandler.OKTyped(req, MyResp{ID: "1"})

// With a custom status:
err := application.RespHandler.OKWithStatusTyped(req, http.StatusCreated, MyResp{ID: "1"})
```

These are convenience wrappers around `OK`/`OKWithStatus` that preserve
type information at the call site. The `any`-based methods remain
available for dynamic response types.

## Step 4: Migrate to Resources

For CRUD endpoints, use the resource pattern. Implement `Resource[ID]`
(each operation returns `handlers.EndpointRuntime`) and register via
`ResourceBuilder` — this is the recommended path for v2 migration.

> **Avoid `TypedResource` for simple resources.** The 14 type parameters
> (7 request + 7 response types) are overkill for simple CRUD + custom
> (non-CRUD) resources like Reload. Use `Resource[ID]` + `ResourceBuilder`
> instead. See [Advanced: TypedResource](#advanced-typedresource-14-type-parameters)
> below for when it's warranted.

Each operation has its own typed request and response. Read operations
(List, Show, New, Edit, Destroy) use `libRequest.NoBinding`; write
operations (Create, Update) use `libRequest.JSON`:

```go
import (
    "github.com/hmmftg/requestCore/libRequest"
    "github.com/hmmftg/requestCore/v2/handlers"
    "github.com/hmmftg/requestCore/v2/resources"
)

type UserResource struct{}

type UserListReq struct{}
type UserListResp struct {
    Users []User `json:"users"`
}

func (r *UserResource) List() handlers.EndpointRuntime {
    return handlers.NewEndpoint[UserListReq, UserListResp](
        "list-users",
        libRequest.NoBinding,
        func(req *UserListReq, trx *handlers.HandlerRequest[UserListReq, UserListResp]) (UserListResp, error) {
            return UserListResp{Users: []User{}}, nil
        },
    )
}

type UserShowReq struct{}
type UserShowResp struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func (r *UserResource) Show() handlers.EndpointRuntime {
    return handlers.NewEndpoint[UserShowReq, UserShowResp](
        "show-user",
        libRequest.NoBinding,
        func(req *UserShowReq, trx *handlers.HandlerRequest[UserShowReq, UserShowResp]) (UserShowResp, error) {
            id, err := resources.GetParsedID[string](trx.V2, "id")
            if err != nil {
                return UserShowResp{}, err
            }
            return UserShowResp{ID: id, Name: "example"}, nil
        },
    )
}

// ... similarly for New, Create, Edit, Update, Destroy
```

### Registration via ResourceBuilder (Recommended)

Register all 7 routes (plus optional custom operations) using the
`ResourceBuilder` fluent API:

```go
err := resources.NewResource[string]("/users").
    EnablePatch().
    WithDefaults(&resources.ResourceDefaults{}).
    Register(application.Router, core, application.RespHandler, &UserResource{})
```

Operations returning nil are registered with a default 405 handler when
`WithDefaults` is set. `EnablePatch()` registers PATCH as an alias for
Update.

### Custom Operations (non-CRUD actions)

For non-standard actions like `Reload` that don't fit the 7 CRUD
operations, use `WithCustom` on the builder:

```go
reloadOp := resources.CustomOperation{
    Method: "POST",
    Path:   "/reload",
    Endpoint: handlers.NewEndpoint[struct{}, ReloadResp](
        "reload-parameters",
        libRequest.NoBinding,
        func(req *struct{}, trx *handlers.HandlerRequest[struct{}, ReloadResp]) (ReloadResp, error) {
            return ReloadResp{Reloaded: true}, nil
        },
    ),
}

err := resources.NewResource[string]("/parameters").
    WithCustom(reloadOp).
    Register(application.Router, core, application.RespHandler, &ParameterResource{})
```

Custom operations are registered before `/{id}` routes to ensure
correct precedence (e.g. `POST /parameters/reload` won't match
`/{id}`).

### Raw Config (Alternative)

`ResourceBuilder` is a thin wrapper around `resources.Register` with
`Config[ID]`. If you prefer explicit configuration:

```go
resources.Register[string](application.Router, resources.Config[string]{
    Path:             "/users",
    Resource:         &UserResource{},
    RespHandler:      application.RespHandler,
    EnablePatchAlias: true,
    Defaults:         &resources.ResourceDefaults{},
})
```

`ResourceBuilder` is preferred for readability — the fluent chain makes
the configuration intent clearer than a struct literal.

### Advanced: TypedResource (14 type parameters)

`TypedResource` is an advanced interface with 14 type parameters
(7 request + 7 response types). Each operation returns a fully typed
`*handlers.Endpoint[Req, Resp]` instead of `handlers.EndpointRuntime`.

**When to use TypedResource:** only when you need the strictest
compile-time guarantees on every operation's request/response types
simultaneously — for example, a shared library resource where callers
must not be able to pass the wrong endpoint type to any operation.

**When NOT to use TypedResource:** for simple CRUD + custom resources
(e.g. a User resource with a Reload action). The 14 type parameters add
verbosity without practical benefit. Use `Resource[ID]` +
`ResourceBuilder` instead.

Any `TypedResource` automatically satisfies `Resource[ID]` because
`*handlers.Endpoint[Req, Resp]` implements `handlers.EndpointRuntime`:

```go
type UserResource struct{}
// Implements TypedResource[string, ListReq, ListResp, ShowReq, ShowResp, ...]
// Each method returns *handlers.Endpoint[Req, Resp] (not EndpointRuntime).
```

## Step 5: Switch Frameworks

v2 makes it trivial to switch frameworks. Just change the `Framework` field:

```go
// Gin
app.Bootstrap(app.Config{Framework: app.FrameworkGin})

// Fiber
app.Bootstrap(app.Config{Framework: app.FrameworkFiber})

// chi (net/http)
app.Bootstrap(app.Config{Framework: app.FrameworkChi})
```

All handlers, resources, and middleware work unchanged across frameworks.

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
    Framework:    app.FrameworkChi,
    SessionStore: store,
    SessionSecret: "your-signing-secret",
})
if err != nil {
    log.Fatal(err)
}

// Add session middleware to a route group
api := application.Register("/api",
    session.Middleware(application.Sessions, "session"),
)
```

For configurable session save failure handling, use
`session.MiddlewareWithConfig` with `SaveStrict` (default, propagates
save failures) or `SaveBestEffort` (logs but does not propagate):

```go
session.MiddlewareWithConfig(session.MiddlewareConfig{
    Manager:         application.Sessions,
    CookieName:      "session",
    SaveFailureMode: session.SaveBestEffort,
})
```

### Typed Session Access

Session values are stored as `any`. Use the generic `GetTyped[T]` and
`SetTyped[T]` accessors for compile-time type safety (no runtime type
assertions):

```go
// Store a typed value
session.SetTyped(sess, "user_id", 42)

// Retrieve with compile-time type checking
userID, err := session.GetTyped[int](sess, "user_id")
if err != nil {
    // key not found or type mismatch
    return err
}
```

The `RequestContext.Session` field is typed as `webFramework.SessionContext`
(an interface), not `any`. You can call `Get`, `Set`, `Delete`, etc.
directly without type-asserting to `*session.Session`.

## Step 7: Add Background Workers

Workers run outside HTTP request contexts but still have full
`webFramework.AddLog` observability. Each job receives a job-owned
`webFramework.WebFramework` backed by a concurrency-safe
`BackgroundParser`:

```go
import (
    "log/slog"
    "github.com/hmmftg/requestCore/webFramework"
    "github.com/hmmftg/requestCore/v2/workers"
)

// Workers are created automatically by Bootstrap.
// Submit jobs:
err := application.Worker.Submit(context.Background(), workers.Job{
    Name: "send-email",
    Handler: func(ctx *workers.JobContext) error {
        // webFramework.AddLog works inside worker jobs.
        // Entries are collected and flushed as a transaction log.
        webFramework.AddLog(ctx.WebFramework, webFramework.HandlerLogTag,
            slog.String("recipient", email))

        // ... send email ...

        return nil
    },
    Options: workers.JobOptions{
        MaxAttempts: 3,
    },
})
```

The worker pool emits mandatory `worker-<name>-req` (success) and
`worker-<name>-req-failed` (failure) log entries after each attempt,
including collected `AddLog` attributes, elapsed time, and sanitized
error text.

## Observability: AddLog is Mandatory

The v2 handler lifecycle calls `webFramework.AddLog` for the handler
title and path. For custom logging within handlers, continue using
`webFramework.AddLog`:

```go
func MyHandler(req *MyReq, trx *handlers.HandlerRequest[MyReq, MyResp]) (MyResp, error) {
    // Log to Splunk transaction pipeline
    webFramework.AddLog(trx.W, "my-handler-step", slog.String("status", "processing"))

    // ... business logic

    // Log API calls (mandatory for external API calls)
    webFramework.AddLog(trx.W, "my-api-call-req", slog.String("url", apiURL))
    // ... make API call
    webFramework.AddLog(trx.W, "my-api-call-req", slog.String("status", "success"))

    return MyResp{}, nil
}
```

**Never** replace `webFramework.AddLog` with `slog.*` or `log.*` —
those do not flow into the Splunk transaction pipeline.

## Using the CLI

Generate scaffolding with the `requestcore` CLI:

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

## Coexistence with v1

v1 and v2 can coexist in the same project. The `go.work` file manages both modules:

```bash
# From the requestCore repo root
go work use . v2
```

This allows importing both v1 and v2 packages simultaneously during migration.

When using `-mod=vendor`, both modules coexist in the `vendor/`
directory. Run `go mod vendor` after adding both dependencies; Go
handles the nested module correctly.

## Periodic Background Tasks (Scheduler)

For long-running poller loops (e.g. periodic data sync, health checks),
use the `workers.Scheduler` instead of the job-submission
`InProcessWorker`. The Scheduler runs a handler at fixed intervals with
the same mandatory `AddLog` observability:

```go
err := application.Scheduler.Schedule(workers.ScheduledJob{
    Name:     "shahkar-analyze",
    Interval: 60 * time.Second,
    Handler: func(ctx *workers.JobContext) error {
        webFramework.AddLog(ctx.WebFramework, webFramework.HandlerLogTag,
            slog.String("status", "polling"))
        // ... poll and process ...
        return nil
    },
})
```

The Scheduler is shut down automatically by `app.Shutdown` alongside
the HTTP server and worker pool.

## FAQ / Troubleshooting

### Go version requirement

Both v1 and v2 now require **Go 1.27.0**. v2 uses generic methods on
`*handlers.Endpoint[Req, Resp]` (e.g. `WithInitializer`, `WithFinalizer`,
`WithPersistence`, `OKTyped[Resp]`), which require Go 1.27+. The v1
module's `go.mod` was updated to 1.27.0 for consistency. If you cannot
upgrade to Go 1.27, use the last v2 prerelease tag before the generics
refactor.

### API call infrastructure (issues.md capabilities)

The 9 capabilities described in the v1 `issues.md` file (base header
builder, instrumented HTTP client, standardized API call logging,
status code extraction, error normalization, timeout guards, retry
framework, URL tracker-ID extraction, skip-pattern matching) are
**already implemented** in the v1 module:

- `handlers.BuildBaseRemoteHeaders` — base header builder
- `libCallApi.NewInstrumentedHTTPClient` — instrumented HTTP client
- `handlers.LogApiCall` — standardized API call logging
- `handlers.ExtractStatusCode` / `handlers.DeriveStatusCode` — status extraction
- `handlers.NormalizeCallError` / `handlers.BuildTimeoutError` — error normalization
- `libRetry` — retry framework with backoff and jitter

These are available to v2 consumers via the v1 import since v2
delegates to v1 for infrastructure.

### Worker audit-trail limitation

Worker jobs run outside HTTP request contexts and do not automatically
insert rows into the `request` persistence table. If your worker jobs
need audit-trail persistence (the same `request` table inserts that
HTTP handlers get), call `core.Responder()` or your persistence layer
manually inside the job handler. This is a known design tradeoff:
worker observability is via `webFramework.AddLog` (Splunk pipeline),
not via the persistence audit trail.

### No tags published yet

If `go get github.com/hmmftg/requestCore/v2@v2.0.0-alpha.0` fails with
"unknown revision", no v2 tags have been published yet. The repository
maintainer must trigger the `Release v2` workflow from the GitHub
Actions tab (manual dispatch) to create the first prerelease tag.

## Checklist

- [ ] Add v2 dependency (pin to a specific prerelease tag)
- [ ] Bootstrap v2 App with legacy core
- [ ] Migrate critical handlers to v2 typed endpoints
- [ ] Register routes via v2 `Router`
- [ ] Migrate CRUD endpoints to `ResourceBuilder` + `Resource[ID]` (not TypedResource)
- [ ] Add custom operations for non-CRUD actions (e.g. Reload)
- [ ] Add session middleware if needed
- [ ] Add background workers (InProcessWorker for jobs, Scheduler for pollers)
- [ ] Verify `webFramework.AddLog` calls in all handlers and worker jobs
- [ ] Use `app.Shutdown` for coordinated HTTP + worker + scheduler shutdown
- [ ] Run cross-framework conformance tests
- [ ] Update CI to test v2 module
- [ ] Verify vendor mode coexistence if using `-mod=vendor`
