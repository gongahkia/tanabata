# Benchmarks

Measured on 2026-05-16 using:

```text
go test ./internal/catalog -run '^$' -bench 'Benchmark(ListQuotes|Search|QuoteProvenance|ProviderSummaries)$' -benchtime=3x -benchmem
```

Environment:

- OS/arch: `darwin/arm64`
- CPU: `Apple M3`
- Dataset: local seeded catalog from legacy plus curated quote fixtures

| Benchmark | Time | Memory | Allocations |
| --- | ---: | ---: | ---: |
| `BenchmarkListQuotes-8` | `858556 ns/op` | `36069 B/op` | `1038 allocs/op` |
| `BenchmarkSearch-8` | `994847 ns/op` | `12896 B/op` | `274 allocs/op` |
| `BenchmarkQuoteProvenance-8` | `505486 ns/op` | `6421 B/op` | `177 allocs/op` |
| `BenchmarkProviderSummaries-8` | `535806 ns/op` | `9232 B/op` | `237 allocs/op` |

## Notes

- These are microbenchmarks for query paths, not end-to-end HTTP latency.
- The current fixture dataset is small, so the benchmark is most useful as a regression guard for allocations and query shape.
- Search should remain below list quotes because FTS reduces candidate rows before quote hydration.
- Provenance and provider summaries are intentionally measured separately because they are prominent demo surfaces.
