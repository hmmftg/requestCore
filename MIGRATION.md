# Migration Guide: v0.28.1 → v1.x

This guide covers upgrading from the last `v0.x` release (`v0.28.1`) to
the stable `v1.x` line of the root module
(`github.com/hmmftg/requestCore`).

## Summary

The v1.0 release stabilizes the root module's import path and public API.
The upgrade is designed to be **non-breaking** for existing consumers:
default behavior, response envelopes, and success statuses remain
unchanged. All new HTTP standards features (RFC 9457 Problem Details,
OAuth no-store headers, conditional requests, Link headers, Retry-After,
idempotency contracts) are **opt-in** and do not alter existing code paths.

## Prerequisites

- **Go 1.27+** is required. The `go.mod` directive was bumped from
  `1.25.5` (v0.28.1) to `1.27.0` in v1.0.
- If you are on an older Go toolchain, update before upgrading
  `requestCore`.

## Import Path

The import path is **unchanged**:

```go
import "github.com/hmmftg/requestCore"
```

Go major version 1 uses the unsuffixed module path. No `/v1` suffix is
needed and no directory rename is required.

## What Changed Since v0.28.1

The root-module delta from `v0.28.1` to `v1.0.0` is limited to
non-breaking infrastructure:

| Area | Change | Consumer impact |
|---|---|---|
| `go.mod` | Go directive `1.25.5` → `1.27.0` | Requires Go 1.27+ toolchain |
| `go.work` | Added workspace including `./v2` | None (workspace is dev-only; consumers use `go.mod`) |
| CI workflows | Added v2 CI, release workflows, lint, architecture checks | None |
| `README.md` | Updated documentation | None |
| `.golangci.yml` | Lint configuration | None |

**No root Go source files changed** between `v0.28.1` and the v1.0
baseline. Existing handler, response, request, query, tracing, and
adapter code is identical.

## Behavioral Compatibility

All default behaviors are preserved:

- **Success status:** `response.OK` continues to use HTTP 200.
- **Response envelope:** The legacy `WsResponse` / `ErrorResponse`
  envelope is unchanged by default.
- **Error format:** Errors continue to use the existing custom envelope.
- **Request headers:** `Request-Id`, `Program-Id`, `Module-Id`,
  `Method-Id`, and `User-Id` headers are unchanged.
- **Observability:** `webFramework.AddLog` remains on every external-call
  and transaction path with the same keys and severity semantics.

## New Opt-In Features (v1.x)

The following standards-oriented features are added as **opt-in** APIs.
They do not change default behavior:

- **RFC 9457 Problem Details** — opt-in problem responder
  (`response/problem.go`) that maps errors to `application/problem+json`
  while delegating successes to the legacy handler.
- **Configurable success status** — `HandlerParameters.SuccessStatus`
  allows 201, 202, 204, etc. Default remains 200.
- **OAuth no-store headers** — `httpsemantics` helper for
  `Cache-Control: no-store` + `Pragma: no-cache` on token responses.
- **RFC 6750 Bearer challenges** — `httpsemantics` helper for
  `WWW-Authenticate` on 401 resource-server responses.
- **Conditional requests** — `httpsemantics` helpers for ETag
  parsing/comparison and `If-Match`/`If-None-Match`/`If-Modified-Since`/
  `If-Unmodified-Since` evaluation.
- **RFC 8288 Link headers** — `httpsemantics` helper for pagination and
  relation link serialization.
- **Retry-After** — `httpsemantics` parser/formatter for delta-seconds
  and HTTP-date forms; opt-in `HonorRetryAfter` in `RetryPolicy`.
- **W3C Trace Context** — `httpsemantics` helpers for inbound
  extraction and outbound injection of `traceparent`/`tracestate`
  headers via the globally configured OpenTelemetry propagator.
- **Idempotency contracts** — `idempotency` package with store interfaces
  and in-memory test store. Application owns persistence and replay
  policy.
- **Cross-version conformance** — `conformance` package with shared
  data-only HTTP conformance vectors for testing v1 and v2 against the
  same RFC requirements.

## How to Upgrade

1. **Update Go** to 1.27+ if you haven't already.
2. **Update the dependency:**

   ```bash
   go get github.com/hmmftg/requestCore@v1.0.0
   go mod tidy
   ```

3. **Run your existing tests.** They should pass without changes.
4. **(Optional)** Adopt opt-in features as needed. See the
   `httpsemantics`, `idempotency`, and `response/problem.go` package
   documentation.

## What Did NOT Change

- No public types, functions, or methods were removed or renamed.
- No method signatures changed.
- No required struct fields were added to existing types.
- The `ResponseHandler` interface is unchanged.
- The `RequestParser` interface is unchanged.
- The `RetryPolicy` defaults (fixed backoff, no Retry-After honoring)
  are unchanged.

## v2 Module

The nested `v2/` module (`github.com/hmmftg/requestCore/v2`) is a
**separate module** with its own `go.mod`, tags (`v2/v2.x.y`), and release
workflow. It is independent of the root v1 release line. See
[v2/MIGRATION.md](v2/MIGRATION.md) for v1-to-v2 migration guidance.

## Questions

If you encounter issues during upgrade, verify:

1. Your Go version is 1.27+.
2. Your `go.mod` does not have a `replace` directive pointing to an old
   path.
3. You are not importing internal packages that moved between v0.x and
   v1.x (none are expected, but check if you imported `internal/` paths).
