# Migrating from v1 to v2

This guide covers migrating existing `requestCore` v1 applications to the v2 module (`github.com/hmmftg/requestCore/v2`).

## Overview

v2 is a **separate Go module** that lives in the `v2/` directory. It is fully backward-compatible — v1 code continues to work unchanged. You can adopt v2 incrementally, package by package.

## Key Changes in v2

| Feature | v1 | v2 |
|---------|----|----|
| Module path | `github.com/hmmftg/requestCore` | `github.com/hmmftg/requestCore/v2` |
| Framework coupling | Gin-specific (`libContext.InitContext`) | Framework-agnostic (`RequestContext`) |
| Response writing | `response.WebHanlder` | `v2/response.Handler` with pluggable renderers |
| Routing | Framework-specific | `routing.Router` interface (Gin, Fiber, chi, net/http) |
| Handlers | `BaseHandler` returns `any` | `BaseHandler` returns `error` |
| Resources | Manual route registration | `resources.Register` with 7 operations |
| Sessions | Not built-in | `session.Manager` with `CookieStore` |
| Workers | Not built-in | `workers.InProcessWorker` with retry |
| CLI | None | `requestcore` CLI for code generation |

## Step 1: Add v2 as a dependency

```bash
cd your-project
go get github.com/hmmftg/requestCore/v2@latest
```

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

### v2 Handler (Framework-agnostic)

```go
import "github.com/hmmftg/requestCore/v2/handlers"

type MyHandler struct{}

func (h *MyHandler) Parameters() handlers.HandlerParameters[MyReq, MyResp] {
    return handlers.HandlerParameters[MyReq, MyResp]{
        Title: "my-handler",
        Path:  "/users",
        Body:  libRequest.JSON,
    }
}

func (h *MyHandler) Initializer(req *handlers.HandlerRequest[MyReq, MyResp]) error {
    return nil
}

func (h *MyHandler) Handler(req *handlers.HandlerRequest[MyReq, MyResp]) (MyResp, error) {
    return MyResp{...}, nil
}

func (h *MyHandler) Finalizer(req *handlers.HandlerRequest[MyReq, MyResp]) {}
func (h *MyHandler) Simulation(req *handlers.HandlerRequest[MyReq, MyResp]) (MyResp, error) {
    return MyResp{}, nil
}

// Registration
h := handlers.BaseHandler[MyReq, MyResp](core, &MyHandler{}, false, application.RespHandler)
application.Router.Get("/users", h)
```

### Simple Endpoint (shorthand)

```go
handlers.GetEndpoint[NoReq, MyResp](application.Router, core, application.RespHandler,
    "/health",
    func(req *NoReq, trx *handlers.HandlerRequest[NoReq, MyResp]) (MyResp, error) {
        return MyResp{Status: "ok"}, nil
    },
)
```

## Step 4: Migrate to Resources

For CRUD endpoints, use the resource pattern. Each operation returns
a `*handlers.Endpoint` with its own typed request and response:

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

func (r *UserResource) List() *handlers.Endpoint {
    return handlers.NewEndpoint[UserListReq, UserListResp](
        "list-users",
        libRequest.JSON,
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

func (r *UserResource) Show() *handlers.Endpoint {
    return handlers.NewEndpoint[UserShowReq, UserShowResp](
        "show-user",
        libRequest.JSON,
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

// Register all 7 routes at once
resources.Register[string](application.Router, resources.Config[string]{
    Path:        "/users",
    Resource:    &UserResource{},
    RespHandler: application.RespHandler,
    Defaults:    &resources.ResourceDefaults{Registry: application.Registry},
})
```

Operations returning nil are registered with a default 405 handler when
`Defaults` is set. Use `EnablePatchAlias: true` to register PATCH as an
alias for Update.

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

store := session.NewCookieStore([]byte("your-32-byte-secret-key"))
application, _ := app.Bootstrap(app.Config{
    Framework:    app.FrameworkChi,
    SessionStore: store,
})

// Add session middleware to a route group
api := application.Register("/api",
    app.SessionMiddleware(application.Sessions, "session"),
)
```

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

The v2 `BaseHandler` calls `webFramework.AddLog` for the handler title and path. For custom logging within handlers, continue using `webFramework.AddLog`:

```go
func (h *MyHandler) Handler(req *handlers.HandlerRequest[MyReq, MyResp]) (MyResp, error) {
    // Log to Splunk transaction pipeline
    webFramework.AddLog(req.W, "my-handler-step", slog.String("status", "processing"))

    // ... business logic

    // Log API calls (mandatory for external API calls)
    webFramework.AddLog(req.W, "my-api-call-req", slog.String("url", apiURL))
    // ... make API call
    webFramework.AddLog(req.W, "my-api-call-req", slog.String("status", "success"))

    return MyResp{}, nil
}
```

**Never** replace `webFramework.AddLog` with `slog.*` or `log.*` — those do not flow into the Splunk transaction pipeline.

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

## Checklist

- [ ] Add v2 dependency
- [ ] Bootstrap v2 App with legacy core
- [ ] Migrate critical handlers to v2 `BaseHandler`
- [ ] Register routes via v2 `Router`
- [ ] Migrate CRUD endpoints to `resources.Register`
- [ ] Add session middleware if needed
- [ ] Add background workers if needed
- [ ] Verify `webFramework.AddLog` calls in all handlers and worker jobs
- [ ] Use `app.Shutdown` for coordinated HTTP + worker shutdown
- [ ] Run cross-framework conformance tests
- [ ] Update CI to test v2 module
