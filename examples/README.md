# requestCore Examples

Runnable demos for the root library. See the root [README.md](../README.md) for full documentation.

| Example | Framework | Port | Run command |
|---|---|---|---|
| [chi-hello](chi-hello/) | chi + net/http | 8080 | `go run ./examples/chi-hello` |
| [gin-hello](gin-hello/) | Gin | 8081 | `go run ./examples/gin-hello` |
| [fiber-hello](fiber-hello/) | Fiber | 8082 | `go run ./examples/fiber-hello` |

Each example exposes the same three routes:

- `GET /health` — liveness check
- `GET /users/{id}` — URL parameter extraction
- `POST /echo` — JSON body parsing and response

## Smoke tests

After starting **chi-hello** on port 8080:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/users/42
curl -X POST http://localhost:8080/echo -d '{"message":"hi"}' -H 'Content-Type: application/json'
```

After starting **gin-hello** on port 8081, replace `8080` with `8081` in the commands above.

After starting **fiber-hello** on port 8082, replace `8080` with `8082` in the commands above.

## Next steps

- **sqlc + database/sql** — see the [Canonical setup](../README.md#canonical-setup-chi--nethttp--sqlc--pgxstdlib) section in the root README
- **Full handler pipeline** — see [handlers/baseHandler.go](../handlers/baseHandler.go) and [handlers/baseHanlder_test.go](../handlers/baseHanlder_test.go)
- **Mock DB testing** — see [testingtools/init.go](../testingtools/init.go) (`InitTestingNoDB`, `libQuery.MockDB`)
