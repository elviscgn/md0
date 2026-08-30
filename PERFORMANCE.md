# Performance

md0 has a built-in Go benchmark harness covering parser, dependency-plan construction, evaluation, HTML rendering, and reactive single-input updates across synthetic 100, 1,000, and 5,000-node documents.

Run it with:

```sh
go test ./internal/md0 -run='^$' -bench='^BenchmarkRuntime$' -benchmem -benchtime=50ms -count=1
```

The CI pipeline runs this benchmark on every main-branch build and verifies that both 5,000-node workloads complete. The benchmark is measurement evidence, not a fixed timing contract: GitHub-hosted runner hardware can vary between runs.

## Workloads

**Chain** creates one input followed by `N` calculations where every value depends on the previous one, ending in a single interpolation. It stresses deep dependency ordering and full-chain incremental invalidation.

**Fan-out** creates one input feeding `N` independent calculations and a prose block that interpolates all `N` results. It stresses wide dependency invalidation and rendering many reactive values.

## Recorded CI snapshot

Recorded snapshot: one GitHub Actions Linux benchmark run; hosted runner hardware and timings can vary.

Environment reported by the runner:

- Ubuntu 24.04 (`ubuntu-24.04`)
- Go 1.23.12
- linux/amd64
- AMD EPYC 9V74 80-Core Processor
- `-benchtime=50ms -count=1`

Times below convert Go's reported `ns/op` to milliseconds per operation.

| Workload | Nodes | Parse | Plan | Eval | Render | Incremental |
|---|---:|---:|---:|---:|---:|---:|
| Chain | 100 | 0.136 ms | 0.213 ms | 0.028 ms | 0.003 ms | 0.063 ms |
| Chain | 1,000 | 1.427 ms | 2.703 ms | 0.416 ms | 0.006 ms | 0.794 ms |
| Chain | 5,000 | 7.745 ms | 16.683 ms | 2.064 ms | 0.020 ms | 4.051 ms |
| Fan-out | 100 | 0.167 ms | 0.352 ms | 0.026 ms | 0.066 ms | 0.059 ms |
| Fan-out | 1,000 | 1.715 ms | 4.053 ms | 0.400 ms | 0.721 ms | 0.762 ms |
| Fan-out | 5,000 | 8.691 ms | 26.055 ms | 1.997 ms | 4.244 ms | 3.854 ms |

### Allocations at 5,000 nodes

| Workload | Stage | Bytes/op | Allocs/op |
|---|---|---:|---:|
| Chain | Parse | 2,011,030 | 40,031 |
| Chain | Plan | 7,629,106 | 35,740 |
| Chain | Eval | 1,540,960 | 147 |
| Chain | Render | 1,832 | 27 |
| Chain | Incremental | 1,966,741 | 5,079 |
| Fan-out | Parse | 2,912,930 | 40,049 |
| Fan-out | Plan | 9,867,037 | 56,087 |
| Fan-out | Eval | 1,540,622 | 147 |
| Fan-out | Render | 1,119,001 | 24,875 |
| Fan-out | Incremental | 2,127,366 | 96 |

## Reading the curve

Going from 100 to 5,000 nodes is a 50× increase in node count. In this snapshot, parse time grew about 52–57×, while plan/eval/incremental stages grew roughly 64–78× depending on workload. That is mildly superlinear rather than perfectly linear, with dependency-plan construction currently the most expensive stage at scale.

The renderer behaves very differently by document shape: the chain only renders one final reactive interpolation, so its 5,000-node render remains tiny; the fan-out report renders thousands of interpolations and reaches about 4.24 ms.

These measurements are intentionally kept separate from correctness limits. We do not fail CI because a shared hosted runner is a few milliseconds slower than a previous one. CI instead fails if the scale benchmark cannot execute the 5,000-node chain or fan-out workloads at all.

## Dogfood report

`examples/runtime-scale.md` contains this measured snapshot as an md0 computational document. The report itself recalculates summaries, renders charts/tables, and asserts a deliberately loose 100 ms planning sanity budget from the recorded values.
