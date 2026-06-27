# Framework-Portable HTTP Handlers in Go with requestCore

**Subtitle:** A practical guide to unified request parsing, JSON handling, and incremental adoption with requestCore.

**Repository:** [github.com/hmmftg/requestCore](https://github.com/hmmftg/requestCore)

**Tags:** Go, Golang, Web Development, REST API, Software Architecture, Open Source, Backend

---

If you've maintained Go services long enough, you've probably seen this pattern:

- Team A builds on **Gin**
- Team B prefers **Fiber**
- Platform standards push **net/http + chi**
- Every service reimplements the same request parsing, JSON binding, error responses, logging, and tracing — differently

Switching frameworks rarely means swapping routers. It means rewriting handlers, middleware assumptions, and test helpers.

**requestCore** takes a different approach: keep your HTTP framework, but normalize request handling behind one interface.

---

## The problem: framework lock-in is deeper than routing

Most Go web frameworks differ in three places that matter for long-lived services:

1. **Context type** — `*gin.Context`, `*fiber.Ctx`, `http.ResponseWriter` + `*http.Request`
2. **Binding** — JSON body, headers, query params
3. **Response helpers** — how you send JSON and status codes

So "we'll migrate from Gin to Fiber later" often becomes a multi-month rewrite — not because routing is hard, but because **handler plumbing** is duplicated and framework-specific.

What you want instead:

- One consistent way to parse requests and write JSON responses
- Freedom to choose Gin, Fiber, or stdlib per service
- Optional layers for DB, audit trails, tracing — without forcing a monolithic framework

---

## The core idea: `RequestParser`

At the center of requestCore is `webFramework.RequestParser` — a single interface for request/response operations:

```go
type RequestParser interface {
    GetMethod() string
    GetPath() string
    GetBody(target any) error
    GetUrlParam(name string) string
    SendJSONRespBody(status int, resp any) error
    // headers, tracing, locals, file upload, and more
}
```

Framework-specific code lives in adapters:

| Framework | Adapter packages |
|---|---|
| Gin | `libGin` |
| Fiber | `libFiber` |
| net/http + chi | `libNetHttp`, `libChi` |

Your handler logic talks to `RequestParser`, not to framework types directly.

---

## Same endpoints, three frameworks

The repository includes three runnable examples with identical routes:

- `GET /health`
- `GET /users/{id}`
- `POST /echo`

### chi + net/http (recommended starting point)

```go
router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
    parser := libChi.InitParser(r, w)
    id := parser.GetUrlParam("id")
    _ = parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id})
})
```

Run:

```bash
go run ./examples/chi-hello
```

Server listens on port **8080**.

### Gin

```go
router.POST("/echo", func(c *gin.Context) {
    wf := libContext.InitContextNoAuditTrail(c)
    var body struct {
        Message string `json:"message"`
    }
    if err := wf.Parser.GetBody(&body); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    _ = wf.Parser.SendJSONRespBody(http.StatusOK, body)
})
```

Run:

```bash
go run ./examples/gin-hello
```

Server listens on port **8081**.

### Fiber

```go
app.Post("/echo", func(c *fiber.Ctx) error {
    wf := libContext.InitContextNoAuditTrail(c)
    var body struct {
        Message string `json:"message"`
    }
    if err := wf.Parser.GetBody(&body); err != nil {
        return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    return wf.Parser.SendJSONRespBody(http.StatusOK, body)
})
```

Run:

```bash
go run ./examples/fiber-hello
```

Server listens on port **8082**.

**Notice the pattern:** framework-specific wiring at the edge; shared `GetBody` / `SendJSONRespBody` in the middle.

### Smoke tests

After starting chi-hello:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/users/42
curl -X POST http://localhost:8080/echo \
  -H 'Content-Type: application/json' \
  -d '{"message":"hi"}'
```

For gin-hello and fiber-hello, use ports **8081** and **8082** respectively.

---

## How framework detection works

`libContext.InitContext` and `InitContextNoAuditTrail` accept `any` and switch on the concrete context type:

```go
switch ctx := c.(type) {
case *gin.Context:
    w.Parser = libGin.InitContext(c)
case *fiber.Ctx:
    w.Parser = libFiber.InitContext(ctx)
case *http.Request:
    w.Parser = libNetHttp.InitContext(...)
// ...
}
```

That gives you a `webFramework.WebFramework` wrapper with a normalized parser — the same shape whether you're in Gin, Fiber, or tests.

### Incremental adoption

You don't have to adopt everything on day one:

| Level | What you use | Best for |
|---|---|---|
| 1 | Parser layer only (`libChi`, `libContext`) | Existing handlers, lowest friction |
| 2 | Request lifecycle (`libRequest`) | Audit trails, duplicate detection |
| 3 | Full pipeline (`handlers.BaseHandler`) | Parse → validate → persist → execute → respond |

---

## Architecture overview

```text
HTTP Framework (Gin / Fiber / chi)
        |
        v
   libContext adapter
        |
        v
  RequestParser interface
        |
        v
  Your handler logic
        |
        +--> libRequest (persistence, duplicate check)
        +--> libQuery (multi-DB execution)
        +--> libTracing / libLogger (observability)
        +--> response (uniform error envelopes)
```

---

## Beyond parsing: why teams reach for requestCore

Framework portability is the entry point. The library also targets repeated backend infrastructure:

| Concern | Package |
|---|---|
| Request audit / persistence | `libRequest` |
| Multi-DB query execution | `libQuery` |
| Uniform error responses | `response` |
| OpenTelemetry tracing | `libTracing` |
| Structured logging (slog, Splunk) | `libLogger` |
| External API calls + OAuth2 | `libCallApi` |

For teams standardizing platforms across many services, that composability matters as much as Gin-vs-Fiber choice.

**Canonical stack in the README:** chi + net/http + sqlc + pgx/stdlib — a low-risk path if you want `database/sql` compatibility without committing to a heavy framework.

---

## When requestCore is a good fit

### Use it when

- Multiple teams or services need shared handler conventions
- You want request persistence, duplicate checking, or audit trails
- You're standardizing observability (OpenTelemetry, slog) across services
- You're migrating frameworks and want to limit handler rewrites

### Skip it when

- You have a small API on one framework with no shared platform needs
- You don't need cross-cutting request infrastructure

requestCore is not another web framework. It's a **request layer** that sits beside Gin, Fiber, or stdlib.

---

## Try it in 2 minutes

```bash
git clone https://github.com/hmmftg/requestCore.git
cd requestCore
go run ./examples/chi-hello
```

In another terminal:

```bash
curl http://localhost:8080/users/42
```

**Further reading:**

- [Examples index](../examples/README.md)
- [pkg.go.dev documentation](https://pkg.go.dev/github.com/hmmftg/requestCore)
- [Root README](../README.md)

---

## What's next

Possible follow-up topics:

1. Adding sqlc + PostgreSQL to a requestCore chi service
2. Request persistence and duplicate detection with `BaseHandler`
3. OpenTelemetry across Gin and Fiber with one tracing model

---

## License note

requestCore is released under **GPL-3.0**. If you're evaluating it for a commercial product, read the license implications carefully. For internal tools, side projects, or GPL-compatible stacks, it may be a good fit — but verify with your team's compliance requirements.

See [LICENSE](../LICENSE) in the repository.

---

## Summary

Every Go team eventually argues about frameworks. The real cost shows up later, when you rewrite JSON binding, header parsing, error envelopes, and tracing because each framework does them differently.

**requestCore** doesn't replace your router. It gives you one `RequestParser` interface across Gin, Fiber, and net/http — so the handler logic you write today survives tomorrow's framework decision.

```bash
go get github.com/hmmftg/requestCore
go run ./examples/chi-hello
```

Star the repo, try the examples, and open an issue if you build something with it: [github.com/hmmftg/requestCore](https://github.com/hmmftg/requestCore)
