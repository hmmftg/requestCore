# v2 Examples

Runnable demos for the v2 module. See [v2/README.md](../README.md) for
full v2 documentation.

## simple

A chi-based example demonstrating the full v2 generics API.

### Run

```bash
go run ./examples/simple
```

The server starts on `http://localhost:8080` (set `PORT` env var to change).

### Endpoints

| Method | Path | Description | Feature Demonstrated |
|--------|------|-------------|----------------------|
| GET | `/health` | Health check | Typed GET endpoint |
| POST | `/users` | Create user | Lifecycle hooks (WithInitializer, WithFinalizer) + background worker |
| GET | `/items` | List items | Resource — List operation |
| GET | `/items/{id}` | Show item | Resource — Show operation |
| GET | `/items/new` | New item form | Resource — New operation |
| POST | `/items` | Create item | Resource — Create + WithPersistence |
| GET | `/items/{id}/edit` | Edit item form | Resource — Edit operation |
| PUT | `/items/{id}` | Update item | Resource — Update operation |
| PATCH | `/items/{id}` | Patch item | Resource — PATCH alias (EnablePatch) |
| DELETE | `/items/{id}` | Delete item | Resource — Destroy operation |
| GET | `/api/profile` | User profile | Session middleware + typed session access (GetTyped/SetTyped) |
| POST | `/echo` | Echo message | Typed POST endpoint |

### Features Demonstrated

- [x] **Generic typed endpoints** — `handlers.GetEndpoint[Req, Resp]`, `handlers.PostEndpoint[Req, Resp]`
- [x] **Lifecycle hooks** — `WithInitializer`, `WithFinalizer` (typed methods on `*Endpoint[Req, Resp]`)
- [x] **WithPersistence** — `NewPersister[Req, Resp]` with insert/update callbacks
- [x] **ResourceBuilder** — `resources.NewResource[string]("/items").EnablePatch().Register(...)`
- [x] **Resource CRUD** — 7 operations (List, Show, New, Create, Edit, Update, Destroy)
- [x] **Session middleware** — `session.Middleware` on a route group
- [x] **Typed session access** — `session.GetTyped[int]`, `session.SetTyped[int]`
- [x] **Background workers** — `application.Worker.Submit` with retry and AddLog
- [x] **webFramework.AddLog** — mandatory observability in all handlers and workers
- [x] **Framework-agnostic** — chi adapter (switch to Gin/Fiber by changing one config field)

### Smoke Tests

```bash
# Health check
curl http://localhost:8080/health

# Create user (lifecycle hooks + worker)
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"alice","email":"alice@example.com"}'

# List items
curl http://localhost:8080/items

# Show item
curl http://localhost:8080/items/1

# Create item (WithPersistence)
curl -X POST http://localhost:8080/items \
  -H 'Content-Type: application/json' \
  -d '{"id":"3","name":"NewItem"}'

# Update item
curl -X PUT http://localhost:8080/items/1 \
  -H 'Content-Type: application/json' \
  -d '{"id":"1","name":"Updated"}'

# Patch item (alias for Update)
curl -X PATCH http://localhost:8080/items/1 \
  -H 'Content-Type: application/json' \
  -d '{"id":"1","name":"Patched"}'

# Delete item
curl -X DELETE http://localhost:8080/items/2

# Echo
curl -X POST http://localhost:8080/echo \
  -H 'Content-Type: application/json' \
  -d '{"message":"hello"}'
```

### Switching Frameworks

Change one line in `main.go` to switch frameworks:

```go
// Gin
app.Bootstrap(app.Config{Framework: app.FrameworkGin, ...})

// Fiber
app.Bootstrap(app.Config{Framework: app.FrameworkFiber, ...})

// chi (current)
app.Bootstrap(app.Config{Framework: app.FrameworkChi, ...})
```

All handlers, resources, and middleware work unchanged.
