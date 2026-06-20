# Handlers Package Documentation

The `handlers` package provides reusable request-processing components for common backend workflows such as:

- querying databases
- executing DML operations
- external API calls
- pagination
- recovery handling
- request consumption pipelines

The package is designed around composable handler logic that can be integrated with the broader `requestCore` ecosystem.

---

# Overview

The handlers package acts as an orchestration layer between:

- request context handling
- query execution
- request lifecycle management
- response generation
- external service communication

It provides reusable building blocks to reduce repetitive endpoint and service logic.

---

# Package Structure

```text
handlers/
├── baseHandler.go
├── callApi.go
├── consumeHandler.go
├── dmlHandler.go
├── ormQueryHandler.go
├── pagination.go
├── persistence.go
├── queryHandler.go
├── recovery.go
└── *_test.go
```

---

# Core Concepts

The handlers package follows several architectural principles:

- reusable request-processing pipelines
- separation of query/DML concerns
- framework-independent business flow handling
- composable handler utilities
- centralized error and recovery behavior
- testing-friendly abstractions

---

# Handlers

## Base Handler

File:
```text
handlers/baseHandler.go
```

The base handler provides common functionality shared across specialized handlers.

Typical responsibilities may include:

- request initialization
- context preparation
- validation flow integration
- shared response handling
- standardized execution behavior
- common logging/tracing hooks

This acts as the foundation for other handler implementations.

---

## Query Handler

File:
```text
handlers/queryHandler.go
```

The query handler is responsible for read-oriented operations.

Typical use cases:

- SELECT queries
- fetching paginated data
- filtering/search operations
- response mapping
- query execution orchestration

This handler integrates with the query layer provided by:

```text
libQuery
```

and may use ORM or direct query execution depending on the implementation.

---

## ORM Query Handler

File:
```text
handlers/ormQueryHandler.go
```

The ORM query handler provides query execution behavior specifically tailored for ORM-backed flows.

Typical responsibilities:

- ORM query execution
- model-based retrieval
- entity mapping
- ORM abstraction integration
- repository-style operations

This handler is useful when the application prefers ORM-based data access instead of raw SQL execution.

---

## DML Handler

File:
```text
handlers/dmlHandler.go
```

The DML handler manages write-oriented database operations.

Typical operations include:

- INSERT
- UPDATE
- DELETE
- transactional workflows
- persistence orchestration

This handler centralizes mutation logic and promotes consistency across write operations.

---

## Call API Handler

File:
```text
handlers/callApi.go
```

The API call handler manages outbound HTTP/API interactions.

Typical responsibilities:

- calling external services
- handling authentication flows
- request serialization
- response parsing
- retry/error handling integration
- multi-service orchestration

This handler works alongside:

```text
libCallApi
```

to standardize external communication behavior.

---

## Consume Handler

File:
```text
handlers/consumeHandler.go
```

The consume handler is intended for request or event consumption workflows.

Possible use cases:

- async processing
- event/message consumption
- queue-driven workflows
- background task handling
- request replay flows

This handler helps encapsulate consumption-oriented execution patterns.

---

## Pagination Utilities

File:
```text
handlers/pagination.go
```

Pagination utilities provide reusable pagination behavior for query endpoints.

Typical capabilities:

- page-based pagination
- offset/limit handling
- response metadata generation
- pagination validation
- standardized paging responses

Useful for:

- REST APIs
- admin panels
- search endpoints
- reporting APIs

---

## Recovery Handler

File:
```text
handlers/recovery.go
```

Recovery utilities provide panic/error recovery behavior for safer request execution.

Typical responsibilities:

- panic recovery
- structured error conversion
- centralized failure handling
- logging unexpected failures
- preventing application crashes from request-level errors

This improves resiliency and operational stability.

---

# Architectural Role

The handlers package sits between:

```text
Transport Layer
    ↓
Context Initialization
    ↓
Handlers
    ↓
Query / Request / Response Layers
    ↓
Database / External Services
```

This creates a reusable execution pipeline that keeps endpoint logic thin and consistent.

---

# Integration with requestCore

Handlers integrate closely with:

| Package | Purpose |
|---|---|
| `libContext` | unified request context |
| `libQuery` | DB/query execution |
| `libRequest` | request lifecycle |
| `response` | standardized responses |
| `libCallApi` | external API communication |
| `libTracing` | observability/tracing |
| `libLogger` | structured logging |

---

# Typical Request Flow

A typical flow using handlers may look like:

```text
Incoming Request
    ↓
Framework Adapter (Gin/Fiber/nethttp)
    ↓
libContext Initialization
    ↓
Handler Execution
    ↓
Query/DML/API Logic
    ↓
Response Generation
```

---

# Observability

Handlers are designed to work with the repository’s observability stack:

- OpenTelemetry tracing
- structured logging
- request-scoped metadata
- framework-aware context propagation

This allows consistent visibility across request pipelines.

---

# Request persistence (optional)

Handlers support optional request/result persistence via `RequestPersister[Req, Resp]` on generic `HandlerParameters[Req, Resp]`.

When `Persistence` is `nil` (default for built-in query/DML/call handlers), no insert or update runs. When set, the framework calls:

1. `Insert(path, req)` after parse — failure aborts the request
2. `Update(path, req)` in `Recovery` after `Finalizer` and log collection — best-effort; errors are logged only and not retried

```go
type RequestPersister[Req, Resp any] interface {
    Insert(path string, req *HandlerRequest[Req, Resp]) error
    Update(path string, req *HandlerRequest[Req, Resp]) error
}
```

### Handler outcome fields

After the response is sent (or after a panic is captured in `Recovery`), `Update` receives a `HandlerRequest` with populated outcome metadata:

```go
type HandlerOutcome struct {
    Error      error // nil on success; set on handler/init/parse/insert errors and panics
    HTTPStatus int   // HTTP status sent or intended (500 on panic); 0 if no response was sent
}

type HandlerRequest[Req, Resp any] struct {
    ...
    Outcome  HandlerOutcome
    Duration time.Duration // elapsed handler time, set in Recovery
    RespSent bool
}
```

`HTTPStatus` is recorded by the response layer (`response.LastHTTPStatusLocal`) after `Responder().OK` / `Responder().Error`, so it reflects the status actually emitted — not a duplicate mapping in handlers. On panic, `Recovery` sets `HTTPStatus` to `500` before `Update` runs.

Use `trx.Outcome.Error` with `libError.Unwrap` to build persistence outcome without reading parser locals such as `errorArray`.

### Record ID handoff

Use a shared local key instead of per-domain names (`shahkar_request_id`, `card_issue_id`, etc.):

```go
handlers.SetPersistedRecordID(req.W, recordID) // any type: int64, string UUID, etc.
id, ok := handlers.GetPersistedRecordID(req.W)
```

### FuncPersister

For tests or thin adapters, implement persistence with function fields:

```go
p := handlers.FuncPersister[MyReq, MyResp]{
    InsertFn: func(path string, req *handlers.HandlerRequest[MyReq, MyResp]) error { ... },
    UpdateFn: func(path string, req *handlers.HandlerRequest[MyReq, MyResp]) error { ... },
}
```

Nil function fields are no-ops. Use `Persistence: nil` when no persistence is needed.

### Example service persister

Consumers implement `RequestPersister` when they need audit storage, for example delegating to `libRequest`:

```go
type ServiceRequestPersister[Req, Resp any] struct{}

func (ServiceRequestPersister[Req, Resp]) Insert(path string, req *handlers.HandlerRequest[Req, Resp]) error {
    err := req.Core.RequestTools().InitRequest(req.W, req.Title, path)
    if err != nil {
        return err
    }
    handlers.SetPersistedRecordID(req.W, req.W.Parser.GetLocal("reqLog").(libRequest.RequestPtr).GetId())
    return nil
}

func (ServiceRequestPersister[Req, Resp]) Update(path string, req *handlers.HandlerRequest[Req, Resp]) error {
    reqLogLocal := req.W.Parser.GetLocal("reqLog")
    reqLog, ok := reqLogLocal.(libRequest.RequestPtr)
    if !ok || reqLog == nil {
        return nil
    }
    reqLog.Outgoing = req.Response
    if req.Outcome.Error != nil {
        if ok, errData := libError.Unwrap(req.Outcome.Error); ok {
            _ = errData // map code/status into your storage model
        }
    }
    return req.Core.RequestTools().UpdateRequestWithContext(req.W.Ctx, reqLog)
}
```

SQL and table shape for the service-tier `request` table live in application setup (`libApplication/env.go`), not in handlers.

---

# Testing

The handlers package includes test coverage through files such as:

```text
baseHanlder_test.go
persistence_test.go
callApi_test.go
consumeHandler_test.go
dmlHandler_test.go
queryHandler_test.go
```

This indicates handlers are designed for isolated testing and reusable execution flows.

---

# Recommended Usage Patterns

## Keep handlers orchestration-focused

Handlers should primarily:

- coordinate flows
- validate execution paths
- call lower-level services
- standardize responses

Avoid embedding heavy business logic directly in handlers.

---

## Use specialized handlers

Prefer:

- `QueryHandler` for reads
- `DMLHandler` for writes
- `CallApiHandler` for external communication

instead of combining unrelated concerns.

---

## Keep business logic independent

Business logic should remain in:

- services
- domain modules
- repositories

while handlers remain execution coordinators.

---

# Suggested Future Enhancements

Potential improvements for the handlers package:

- middleware chaining support
- generic handler pipelines
- transactional scopes
- retry policies
- circuit breaker integration
- async workflow helpers
- event-driven handler abstractions
- CQRS-oriented handler separation

---

# Summary

The `handlers` package provides reusable orchestration components for:

- query execution
- DML operations
- external API communication
- pagination
- recovery
- request consumption flows

It serves as a reusable execution layer that helps standardize backend request processing while remaining framework-independent and observability-friendly.