# Contributing to requestCore

Thanks for your interest in contributing to requestCore. This document covers setup, testing, linting, and the conventions the project follows.

## Project layout

requestCore contains **two independent Go modules**:

- **Root (v1)** — `github.com/hmmftg/requestCore`, the stable line. Lives at the repository root.
- **v2** — `github.com/hmmftg/requestCore/v2`, a generics-first alpha. Lives under `v2/` with its own `go.mod`.

Both modules are part of a single [Go workspace](https://go.dev/ref/mod#workspaces) (`go.work`). Most development touches one module at a time.

## Prerequisites

- **Go 1.27+**
- A supported SQL database driver if you're testing the query layer against a real DB (the mock DB mode needs no driver)
- `golangci-lint` for local linting (the CI workflow pins its own version; see `.github/workflows/lint.yml`)

## Setup

```bash
git clone https://github.com/hmmftg/requestCore.git
cd requestCore
go work sync        # sync the workspace
go mod download     # download dependencies for both modules
```

## Common commands

The `Makefile` wraps the most common tasks:

```bash
make test      # run go test ./...
make build     # build the root module
make fmt       # go fmt ./...
make lint      # go vet ./...
make check     # fmt + lint + test
make dev       # check + build
```

For the v2 module, run Go commands directly inside `v2/`:

```bash
cd v2
go test ./...
go vet ./...
```

### Linting

The project uses `golangci-lint` (config in `.golangci.yml`) with `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosec`, `misspell`, and `revive`. The CI lint workflow is the source of truth — run it locally before pushing:

```bash
golangci-lint run
```

### Running the examples

```bash
go run ./examples/chi-hello     # chi + net/http, port 8080
go run ./examples/gin-hello    # Gin, port 8081
go run ./examples/fiber-hello  # Fiber, port 8082
```

See [examples/README.md](examples/README.md) for smoke-test commands.

## Commit conventions

This project uses **Conventional Commits**. The versioning workflow (`.github/workflows/auto-version.yml`) parses commit subjects to determine version bumps, so the prefix matters:

| Prefix | Triggers | Example |
|---|---|---|
| `feat`, `feature`, `add` | minor bump | `feat(idempotency): add dedup contract` |
| `fix`, `bug`, `patch`, `docs`, `style`, `refactor`, `perf`, `test`, `chore` | patch bump | `fix(libQuery): map duplicate-key error` |
| `feat!` / `BREAKING CHANGE` | major bump | `feat(libRequest)!: change Init signature` |

- Keep the subject line under ~72 characters.
- Scope is optional but encouraged for changes confined to one package (e.g. `feat(v2/workers): ...`).
- v2-only commits do **not** trigger root-module versioning — the workflow scopes bumps to root-module files.

## Pull requests

1. Fork the repo and create a branch from `main`.
2. Make your change. Add or update tests where relevant.
3. Run `make check` (root) and `go test ./...` (v2) locally.
4. Open a PR against `main` with a clear description of what and why.
5. CI runs build, lint, and tests on your PR. Please make sure they're green before requesting review.

## Areas that welcome contributions

- **Framework adapters** — new HTTP framework support (Echo, stdlib router, etc.)
- **Documentation** — guides under `docs/`, examples, and the README
- **Observability** — tracing and logging enhancements
- **Request lifecycle helpers** — duplicate detection, persistence flows
- **Database integrations** — expanding the `libQuery` DB mode matrix
- **Tests and examples** — especially cross-framework parity tests

## Manual setup checklist (one-time, repo maintainer)

These are GitHub repository settings, not code, so they're listed here for reference:

- [ ] Enable **Discussions** in Settings → General → Features
- [ ] Set repository **Topics** (see the footer of [README.md](README.md))
- [ ] Enable **Security Advisories** for vulnerability reports (see [SECURITY.md](SECURITY.md))

## Code of conduct

Participation in this project is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Please be respectful and constructive.
