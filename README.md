
# requestCore

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

- Go 1.x
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

## Testing

The repository includes a strong testing story:

- framework-aware testing support
- mock DB mode
- fake API helpers
- package-level unit tests
- testing context support

This allows request handling, query execution, and framework adapters to be tested independently.

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
2. a **more enterprise-style README**
3. a **README with real code examples** based on the exported APIs
4. a **comparison section vs Kratos / Gin / Fiber / composable stack**
