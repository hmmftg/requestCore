# Changelog

All notable changes to requestCore are documented here. This file is
maintained by hand for high-level milestones; per-release commit detail is
generated automatically in [GitHub Releases](https://github.com/hmmftg/requestCore/releases).

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

This repository contains two independent modules with separate release streams
(see the root [README](README.md#release-lines)):

- **Root (v1)** — `github.com/hmmftg/requestCore`, tags `v0.x.y` / `v1.x.y`
- **v2** — `github.com/hmmftg/requestCore/v2`, tags `v2/v2.0.0-alpha.N`

---

## [Unreleased]

### Added
- Discoverability pass: community files (CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, CITATION), restructured README, and `docs/` folder for guides.

---

## Root module (v1)

### v0.29.0 — 2026-09-08

#### Added
- RFC 9457 problem responses, status-aware success responses, and OAuth/Bearer helpers (`feat(response)`)
- Conditional requests, `Link`, `Retry-After`, and Trace Context support (`feat(http)`)
- Idempotency contracts and execution (`feat(idempotency)`)
- Cross-version conformance tests, docs, and release readiness (`feat(conformance)`)

#### Changed
- Release safety and stable-v1 baseline (`feat(ci)`)

### v0.28.1 — 2026-08-11

#### Notes
- Last release on the v0.28.x line before the v0.29 feature set. See [MIGRATION.md](MIGRATION.md) for the v0.28.1 → v1.x upgrade guide.

---

## v2 module

### v2.0.0-alpha.2 — 2026-09-06

#### Added
- `remotecall` package with Resty adapter, backoff, jitter, body-based retry, and `Retry-After` support (`feat(v2/remotecall)`)

### v2.0.0-alpha.1

#### Added
- Tranche 6 Phase 1: `remotecall` package with Resty adapter and architecture gates (`feat(v2)`)

### v2.0.0-alpha.0

#### Notes
- First v2 alpha tag. Generics-first kernel: typed endpoints, framework-neutral routing, RFC 9457 problem responses, structured telemetry via `slog`, and typed session access. See [v2/README.md](v2/README.md).

---

For the full commit-level history of each release, see the
[GitHub Releases page](https://github.com/hmmftg/requestCore/releases).
