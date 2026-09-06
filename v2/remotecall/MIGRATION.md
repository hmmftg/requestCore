# v1 → v2 Remote Call Migration Guide

## Overview

The v2 `remotecall` package replaces the v1 `libCallApi`/`handlers` remote call stack with a clean, generic, policy-owned API backed by [go-resty/resty/v3](https://github.com/go-resty/resty) (hidden behind an internal adapter).

**Key principle**: requestCore owns all policy (retry, circuit breaking, error classification, telemetry, auth, response building). The adapter owns only HTTP mechanics.

## Quick Start

```go
import (
    "github.com/hmmftg/requestCore/v2/remotecall"
    _ "github.com/hmmftg/requestCore/v2/internal/restyadapter" // registers default adapter
)

client := remotecall.NewRemoteClient(
    remotecall.WithAppName("my-service"),
    remotecall.WithAuthProvider(remotecall.BearerToken{Token: "abc123"}),
    remotecall.WithDefaultRetry(remotecall.RetryPolicy{
        MaxRetries:       2,
        RetryableMethods: map[string]bool{"GET": true},
        RetryOnStatus:    map[int]bool{502: true, 503: true},
        RetryOnTransport: true,
        Backoff: &remotecall.BackoffPolicy{
            InitialDelay: 2 * time.Second,
            MaxDelay:     5 * time.Minute,
            Multiplier:   2.0,
            JitterFactor: 0.2, // ±20% jitter
        },
    }),
    remotecall.WithCircuitBreaker(remotecall.CircuitBreakerPolicy{
        FailureThreshold: 5,
        OpenDuration:     60 * time.Second,
    }),
)

type MyRequest struct {
    Name string `json:"name"`
}
type MyResponse struct {
    ID string `json:"id"`
}

resp, err := client.Call(context.Background(), remotecall.CallConfig[MyRequest, MyResponse]{
    API:      remotecall.RemoteAPI{Name: "user-api", BaseURL: "https://api.example.com"},
    Method:   "POST",
    Path:     "/users",
    BodyType: remotecall.BodyTypeJSON,
    Body:     MyRequest{Name: "alice"},
    Builder:  remotecall.DefaultBuilder[MyResponse]{},
})
```

## API Mapping

| v1 API | v2 Replacement | Notes |
|--------|----------------|-------|
| `handlers.CallAPIJSON` | `remotecall.RemoteClient.Call` | Direct replacement |
| `handlers.CallAPIJSONWithOpts` | `remotecall.RemoteClient.Call` with `CallConfig` | Retry/circuit are new |
| `handlers.BuildBaseRemoteHeaders` | `remotecall.BuildBaseHeaders` | Uses `HeaderContext` |
| `handlers.ShouldSkipAPICall` | `remotecall.ShouldSkipCall` | Patterns moved to `RemoteAPI.SkipPatterns` |
| `handlers.BuildTimeoutError` | `remotecall.BuildTimeoutError` | Direct replacement |
| `handlers.NormalizeCallError` | `remotecall.NormalizeCallError` | Direct replacement |
| `libCallApi.RemoteAPI` | `remotecall.RemoteAPI` | Auth simplified to `AuthProvider` |
| `libCallApi.RemoteCallError` | `remotecall.RemoteCallError` | Adds `ErrorKind` classification |
| `libCallApi.ExtractTrackerID` | `remotecall.ExtractTrackerID` | Direct replacement |
| `libCallApi.NewInstrumentedHTTPClient` | `remotecall.NewInstrumentedClient` | Direct replacement |
| `libCallApi.Auth` / `OAuth2Token` / `TokenCache` | `AuthProvider` interface | OAuth2 moved out of core |
| `libRetry.RetryPolicy` / `WithRetry` | `remotecall.RetryPolicy` | Resty executes the retry loop |
| `libTracing.HTTPClientMetricsRecorder` | `remotecall.MetricsRecorder` | Interface replacement |
| `webFramework.AddLog` | `telemetry.Sink.Record` | Structured events |

## Key Differences

### 1. Generic typed Call
v2 `Call[Req, Resp]` is generic — no `interface{}` casting. The `ResponseBuilder[Resp]` handles unmarshalling.

### 2. Retry is opt-in
v1 retried by default. v2 requires explicit `RetryPolicy` (via `WithDefaultRetry` or per-call `CallConfig.Retry`).

### 3. Circuit breaker is new
v2 introduces per-API+method circuit breaking. Opt-in via `WithCircuitBreaker`.

### 4. Auth is an interface
v1 embedded auth types. v2 uses a single `AuthProvider` interface with `Apply(ctx, headers)`. OAuth2 is external.

### 5. Error classification
v2 `RemoteCallError` has `ErrorKind` (HTTP, Transport, Timeout, Context, Decode) with `errors.Is`/`errors.As` support and `ToProblem()` for RFC 9457.

### 6. No webFramework dependency
v2 uses `telemetry.Sink` instead of `webFramework.AddLog`. Context is first-class `context.Context`.

### 7. Form bodies must be `url.Values`
When `BodyType == Form`, `Body` must have dynamic type `url.Values`. Other map types are rejected at preflight.

### 8. Backoff and jitter
v2 supports declarative backoff via `BackoffPolicy` on `RetryPolicy`. The adapter translates it into a Resty `RetryDelayStrategyFunc`. Supports exponential backoff (`Multiplier`), max delay cap (`MaxDelay`), and jitter (`JitterFactor` 0.0–1.0).

**Cartino reversal worker example** (2s initial, 5min max, factor 2, ±20% jitter):
```go
Backoff: &remotecall.BackoffPolicy{
    InitialDelay: 2 * time.Second,
    MaxDelay:     5 * time.Minute,
    Multiplier:   2.0,
    JitterFactor: 0.2,
}
```

### 9. Retry on response body content
v2 supports retry decisions based on response body content via `RetryOnBody` callback. This covers Galaxy's `ExceptionDetail.Key == "SERVICE_UNAVAILABLE"` pattern:
```go
RetryOnBody: func(statusCode int, body []byte) bool {
    return statusCode == 200 && strings.Contains(string(body), "SERVICE_UNAVAILABLE")
},
```

### 10. Rate-limit retry with Retry-After
v2 supports honoring the `Retry-After` HTTP header for custom status codes via `HonorRetryAfter`. Resty's built-in behavior only honors `Retry-After` for 429/503. When `HonorRetryAfter` is true, the adapter extends this to all status codes in `RetryOnStatus`:
```go
Retry: &remotecall.RetryPolicy{
    MaxRetries:       1,
    RetryableMethods: map[string]bool{"GET": true},
    RetryOnStatus:    map[int]bool{406: true, 330: true},
    HonorRetryAfter:  true,
},
```

For Keyhan's `time.Sleep(1 * time.Second)` pattern, either use `HonorRetryAfter` with a server-side `Retry-After: 1` header, or use a `BackoffPolicy` with `InitialDelay: 1 * time.Second`.

## Gap Resolution Status

| Gap | Status | Solution |
|-----|--------|----------|
| Backoff/jitter control | **Resolved** | `BackoffPolicy` struct on `RetryPolicy` |
| Retry-on-error-key (body content) | **Resolved** | `RetryOnBody` callback on `RetryPolicy` |
| Rate-limit retry with sleep | **Resolved** | `HonorRetryAfter` + `BackoffPolicy` for fixed delay |
| External-call persistence | **Manual via `OnAttempt`** | Wire `OnAttempt` hook + `telemetry.Sink` |
| OAuth2/token refresh | **External `AuthProvider`** | Implement custom `AuthProvider` |
| Alpha maturity | **Pending** | Awaiting v2.0.0 stable + resty v3 stable release |

## Not Supported in v2

- `libCallApi.MultiCall` — build on top of `Call` if needed
- `libCallApi.TransmitSoap` — intentional removal
- `ConsumeRestBasicAuthAPI` / `ConsumeRestAPI` — replaced by typed `Call`
