// Package requestcore is the v2 module of requestCore.
//
// v2 is a **generics-first**, framework-agnostic HTTP application toolkit
// that builds on the root [github.com/hmmftg/requestCore] module while
// preserving full backward compatibility. It requires **Go 1.27+** for
// generic methods on [handlers.Endpoint].
//
// # Core Features
//
//   - **Generic typed endpoints** — [handlers.Endpoint[Req, Resp]] flows
//     request and response types through the entire lifecycle (parse,
//     initialize, handle, render, finalize) without type erasure.
//     Lifecycle hooks ([handlers.Endpoint.WithInitializer],
//     [handlers.Endpoint.WithFinalizer],
//     [handlers.Endpoint.WithPersistence]) are fully typed methods —
//     no reflection, no runtime type-mismatch panics.
//
//   - **Generic resources** — [resources.Resource[ID]] defines 7 CRUD
//     operations where ID is constrained to [cmp.Ordered] (string, int,
//     int64, etc.). The recommended path for v2 migration is
//     [resources.Resource[ID]] paired with [resources.ResourceBuilder[ID]]
//     for fluent registration. [resources.TypedResource] (14 type
//     parameters) is an advanced alternative for cases requiring the
//     strictest per-operation type guarantees — it is overkill for
//     simple CRUD + custom resources.
//
//   - **Typed session access** — [session.GetTyped[T]] and
//     [session.SetTyped[T]] provide compile-time type-safe session value
//     access. The [webFramework.SessionContext] and
//     [webFramework.FlashContext] interfaces eliminate `any` type
//     assertions in handlers.
//
//   - **Generic response helpers** — [response.Handler.OKTyped[Resp]]
//     and [response.Handler.OKWithStatusTyped[Resp]] render typed
//     responses without `any` parameters.
//
//   - **Framework-agnostic routing** — [routing.Router] and
//     [routing.RouteGroup] interfaces work across Gin, Fiber, chi, and
//     net/http via adapter packages ([libGin], [libFiber], [libChi],
//     [libNetHttp]).
//
//   - **Pluggable renderers** — [renderers.Renderer] interface with
//     built-in JSON, XML, text, and CSV renderers.
//
//   - **Error handler registry** — [response.Registry] with per-status
//     handlers and legacy fallback.
//
//   - **Bounded worker pool** — [workers.InProcessWorker] with retry,
//     tracing, and mandatory [webFramework.AddLog] observability.
//
//   - **Scheduler** — [workers.Scheduler] for periodic background tasks.
//
//   - **CLI code generators** — `requestcore` CLI generates handlers,
//     resources, middleware, and project scaffolding.
//
// # Module Structure
//
// The v2 module lives in the v2/ directory and imports the root module
// for delegation to existing query, persistence, response, logging, and
// tracing infrastructure. The root module never imports v2.
//
// See [MIGRATION.md] for the v1-to-v2 migration guide and [README.md]
// for the v2 module overview.
//
// [MIGRATION.md]: https://github.com/hmmftg/requestCore/blob/main/v2/MIGRATION.md
// [README.md]: https://github.com/hmmftg/requestCore/blob/main/v2/README.md
package requestcore
