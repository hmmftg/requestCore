# requestCore

[![Go Reference](https://pkg.go.dev/badge/github.com/hmmftg/requestCore.svg)](https://pkg.go.dev/github.com/hmmftg/requestCore)
[![Release](https://img.shields.io/github/v/release/hmmftg/requestCore)](https://github.com/hmmftg/requestCore/releases)
[![License](https://img.shields.io/github/license/hmmftg/requestCore)](https://github.com/hmmftg/requestCore/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/hmmftg/requestCore)](https://go.dev/dl/)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/hmmftg/requestCore)

`requestCore` is a Go library for handling RESTful requests with a **framework-agnostic core** and adapters for **Gin**, **Fiber**, and **net/http**. It provides a unified request/context layer, query execution abstractions, response handling, logging, tracing, and testing utilities.

It is designed to reduce boilerplate around request processing while keeping the implementation **composable**, **interface-driven**, and **portable across web frameworks**.

---

## Features

- **Framework adapters**
  - Gin
  - Fiber
  - net/http
  - testing support

- **Unified request context**
  - normalized access to framework context
  - request metadata extraction
  - trace propagation
  - user identity handling

- **Query and DB abstraction**
  - multi-database support
  - query runner abstraction
  - mock database mode for tests

- **Request lifecycle helpers**
  - request initialization
  - duplicate request detection
  - request insert/update flows
  - context-aware request operations

- **Structured logging**
  - `slog`-based logging support
  - framework-aware logger integrations
  - Splunk-oriented logging support

- **OpenTelemetry support**
  - trace extraction and propagation
  - request context instrumentation
  - observability-friendly design

- **Testing utilities**
  - fake/mock infrastructure
  - testing-aware context initialization
  - mock DB mode

- **Additional utilities**
  - validation helpers
  - response helpers
  - error handling
  - crypto/security helpers
  - HTTP API calling utilities
  - Swagger-related support

---

## Project goals

`requestCore` is intended to provide a reusable foundation for handling request-oriented backend workflows such as:

- request parsing and normalization
- request logging and tracing
- request persistence and duplicate checking
- database query execution
- consistent response generation
- integration with multiple HTTP frameworks

The codebase is organized around small interfaces and adapter packages rather than a single large runtime framework.

---

## Architecture overview

The repository is centered around a thin root façade and multiple focused subpackages:

### Root façade
- `requestCore.go`
  - exposes the main `RequestCoreModel`
  - provides access to:
    - DB/query runner
    - ORM interface
    - request tools
    - response handler
    - parameter interface

### Core layers
- `libContext`
  - framework-aware context initialization
  - tracing extraction
  - user and framework metadata handling

- `libRequest`
  - request lifecycle and persistence operations
  - initialization paths with and without logging
  - duplicate detection
  - context-aware updates

- `libQuery`
  - query runner abstraction
  - DB mode handling
  - execution helpers
  - ORM-oriented query support

- `response`
  - response handling
  - error response modeling
  - sanitization and web handler support

- `libParams`
  - parameter modeling and loading
  - networking, logging, DB, and security parameters

### Framework adapters
- `libGin`
- `libFiber`
- `libNetHttp`
- `webFramework`

### Supporting packages
- `libLogger`
- `libTracing`
- `libError`
- `libValidate`
- `libCallApi`
- `libCrypto`
- `handlers`
- `swagger`
- `testingtools`

---

## Supported web frameworks

`requestCore` currently supports:

- **Gin**
- **Fiber**
- **net/http**
- **testing**

---

## Requirements

- Go 1.25+
- A supported SQL database driver, depending on your chosen DB mode
- Optional:
  - OpenTelemetry
  - structured logging backend
  - ORM integration

---

## Installation

```bash
go get github.com/hmmftg/requestCore
```

Then import the package in your project:

```go
import "github.com/hmmftg/requestCore"
```

---

## Quick start

> The exact initialization pattern depends on which framework adapter you use.  
> The example below shows the intent of the library: create a core model and use it to access request, query, response, and parameter services.

```go
package main

import (
	"fmt"

	requestcore "github.com/hmmftg/requestCore"
)

func main() {
	core := requestcore.NewRequestCore()

	fmt.Println(core != nil)
}
```

If your project initializes through a specific framework context, you would typically:

1. create the request context
2. initialize `libContext`
3. access request/query/response services through the root model

---

## Package map

### `requestCore.go`
Root façade exposing the main interfaces.

### `libContext`
Detects and normalizes framework context, including:

- Gin
- Fiber
- net/http
- testing

Also integrates tracing metadata and user identity extraction.

### `libRequest`
Request operations such as:

- initialization
- duplicate checking
- request insertion
- updates with context
- no-log initialization path

### `libQuery`
Database/query layer with support for multiple DB modes, including:

- Oracle
- PostgreSQL
- SQLite
- MySQL
- Mock DB

### `response`
Response generation and error handling utilities.

### `libLogger`
Logging utilities, including `slog` and Splunk-oriented integrations.

### `libTracing`
OpenTelemetry-related tracing and instrumentation helpers.

### `libValidate`
Input validation helpers.

### `libCallApi`
Utilities for calling external APIs and handling auth/multi-call scenarios.

Remote APIs can authenticate with OAuth2 (`client_credentials`, `refresh_token`, optional `password` grant) or fall back to BasicAuth when `grant-type` is not configured.

Example `param.yaml`:

```yaml
remoteApis:
  partner-api:
    domain: https://api.partner.com
    name: partner-api
    auth:
      grant-type: client_credentials
      auth-uri: https://auth.partner.com/oauth/token
      client-id: partner-client
```

Secure values (existing pattern):

- `remote-api#partner-api#client-secret`
- `remote-api#partner-api#client-id`
- `remote-api#partner-api#auth-uri` (alias: `auth-url`)

### `libCrypto`
Cryptographic and security primitives.

### `handlers`
Reusable handler implementations for request, query, DML, pagination, recovery, and API call flows.

### `testingtools`
Test helpers, mocks, and simulation utilities.

---

## Observability

`requestCore` is observability-friendly and includes support for:

- trace context extraction
- OpenTelemetry integration
- framework-aware logging
- structured logs via `slog`
- framework-specific logging adapters

This makes it suitable for services that need request-level visibility without hard-coding observability into business logic.

---

## Database support

The query layer supports multiple DB modes, including:

- Oracle
- PostgreSQL
- SQLite
- MySQL
- Mock DB

This makes the library suitable for heterogeneous environments and for testing without a real database.

---

## Canonical setup: chi + net/http + sqlc + pgx/stdlib

For low-risk adoption, use `sqlc` in `database/sql` mode and connect PostgreSQL with pgx stdlib.

### 1) sqlc configuration

```yaml
version: "2"
sql:
  - schema: "db/schema.sql"
    queries: "db/query.sql"
    engine: "postgresql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "database/sql"
```

### 2) Open DB with pgx stdlib

```go
import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

db, err := sql.Open("pgx", "postgres://user:pass@localhost:5432/appdb?sslmode=disable")
if err != nil {
	panic(err)
}
defer db.Close()
```

### 3) Use chi route params with requestCore net/http parser

```go
router := chi.NewRouter()
router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
	parser := libChi.InitParser(r, w)
	id := parser.GetUrlParam("id")
	_ = parser.SendJSONRespBody(http.StatusOK, map[string]string{"id": id})
})
```

This path keeps compatibility with the current `database/sql`-oriented query layer and enables incremental adoption.

---

## Testing

The repository includes a strong testing story:

- framework-aware testing support
- mock DB mode
- fake API helpers
- package-level unit tests
- testing context support

This allows request handling, query execution, and framework adapters to be tested independently.

---

## Optional advanced path: pgx-native sqlc mode

If you need `sqlc` generated code for `pgx/v5` native interfaces (instead of `database/sql`), treat it as a separate compatibility track:

- keep current `QueryRunnerInterface` (`database/sql`) for backward compatibility
- add a parallel pgx-native runner contract and adapter implementation
- maintain parity tests for both backends:
  - query behavior and error mapping
  - DML behavior
  - tracing/logging hooks

Suggested parity matrix:

| Capability | database/sql backend | pgx-native backend |
|---|---|---|
| Single-row query mapping | required | required |
| Multi-row query mapping | required | required |
| DML affected rows handling | required | required |
| Duplicate / no-data error mapping | required | required |
| Request-scoped tracing attributes | required | required |
| Existing handlers compatibility | required | required |

This minimizes risk for existing users while allowing pgx-native optimization where needed.

---

## Design principles

`requestCore` appears to follow these principles:

- **composition over inheritance**
- **framework portability**
- **interface-driven design**
- **explicit abstractions**
- **observability by default**
- **testability first**

---

## Repository structure

```text
requestCore/
├── requestCore.go
├── libContext/
├── libRequest/
├── libQuery/
├── libParams/
├── response/
├── libLogger/
├── libTracing/
├── libValidate/
├── libCallApi/
├── libCrypto/
├── handlers/
├── swagger/
├── testingtools/
├── libGin/
├── libFiber/
├── libNetHttp/
└── webFramework/
```

---

## Documentation

Additional documentation included in the repository:

- `OPENTELEMETRY_INTEGRATION.md`
- `NETHTTP_IMPLEMENTATION_COMPLETE.md`
- `DYNAMIC_HEADERS_GUIDE.md`
- `VERSIONING.md`
- `VERSIONING_SETUP.md`
- `SETUP_COMPLETE.md`

---

## Contributing

Contributions are welcome.

Suggested areas for contribution:

- framework adapters
- documentation
- observability enhancements
- request lifecycle helpers
- database integrations
- tests and examples

---

## License

See `LICENSE` for license information.

---
