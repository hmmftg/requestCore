# Benchmark Baseline — Tranche 0

## Environment

- **OS:** windows/amd64
- **CPU:** 11th Gen Intel(R) Core(TM) i5-11400 @ 2.60GHz (12 logical cores)
- **Go:** go1.27.0
- **Date:** 2026-08-26

## Methodology

- 10 runs per benchmark (`-count=10`)
- 100 iterations per run (`-benchtime=100x`)
- Benchmarks run with `-run=^$` (no tests, benchmarks only)
- Results saved to `baseline_tranche0_handlers.txt` and `baseline_tranche0_routing.txt`

## Baseline Results (median ns/op)

### Handler Benchmarks (Gin adapter)

| Benchmark | Median ns/op |
|-----------|-------------|
| BenchmarkEndpointDirectSuccess | ~7,500 |
| BenchmarkEndpointDirectProblem | ~12,200 |
| BenchmarkEndpointFiveMiddleware | ~7,000 |
| BenchmarkBindJSON | (see file) |
| BenchmarkRenderJSON | (see file) |
| BenchmarkEnterpriseAddLogSuccess | (see file) |

### Adapter Benchmarks (one HTTP round trip)

| Benchmark | Median ns/op |
|-----------|-------------|
| BenchmarkAdapterGin | ~970 |
| BenchmarkAdapterFiber | ~12,000 |
| BenchmarkAdapterChi | ~5,000 |
| BenchmarkAdapterNetHTTP | ~3,500 |

## Gate

Controlled benchmark comparisons in later tranches may not regress by more
than 5% (median time) without a reviewed release-blocking exception. CI
reports gross regressions and allocation changes; the hard 5% gate runs on
a controlled release environment.
