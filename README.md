<p align="center">
  <img src="assets/md0-banner.svg" alt="md0 — documents that compute without becoming software" width="100%">
</p>

<div align="center">

[![CI](https://github.com/elviscgn/md0/actions/workflows/ci.yml/badge.svg)](https://github.com/elviscgn/md0/actions/workflows/ci.yml)
[![Markdown](https://img.shields.io/badge/Markdown-native-231F20?style=for-the-badge&logo=markdown&logoColor=white)](#)
[![PURE](https://img.shields.io/badge/md0%2FPURE-bounded-C25A2B?style=for-the-badge&logo=letsencrypt&logoColor=white)](SECURITY.md)
[![Dependencies](https://img.shields.io/badge/runtime_deps-0-2E7D4F?style=for-the-badge&logo=dependabot&logoColor=white)](STDLIB.md)
[![Status](https://img.shields.io/badge/status-v0.1.0-7A6E67?style=for-the-badge&logo=readthedocs&logoColor=white)](#)

</div>

md0 is a zero-dependency runtime for **interactive Markdown that stays a document**: typed inputs, calculations, reactive prose, conditions, tables, charts, and assertions — without giving the document arbitrary code execution.

> **Documents should be able to compute without becoming software.**

## Try the flagship example

Building from source requires Go 1.27 or newer.

```bash
make build
./bin/md0 open examples/parser-benchmark.md
```

Change the candidate benchmark from `1.31` to `2.10` in the browser. md0 invalidates only the affected dependency-graph branch, recomputes the dependent values, and patches the affected prose/chart/assertion regions without replacing the whole document.

```md
# Parser Benchmark

Baseline: @input baseline_ms number = 1.82
Candidate: @input candidate_ms number = 1.31

@calc change = (baseline_ms - candidate_ms) / baseline_ms * 100

Parse time **{{ change >= 0 ? "improved" : "regressed" }}
by {{ round(abs(change) * 10) / 10 }}%**.

@assert candidate_ms <= baseline_ms * 1.05
Candidate regressed by more than the allowed 5%.
@end

@chart latency
labels = ["Baseline", "Candidate"]
values = [baseline_ms, candidate_ms]
@end
```

The artifact remains readable Markdown. md0 supplies computation when you want it.

## The missing middle

| | Markdown | **md0** | Application |
|---|:---:|:---:|:---:|
| Prose-first | ✓ | ✓ | — |
| Reactive computation | — | ✓ | ✓ |
| Charts + assertions | — | ✓ | ✓ |
| Arbitrary document code | — | **—** | ✓ |
| Package/build stack required | — | **—** | usually |
| Single readable artifact | ✓ | **✓** | usually not |

**md0 keeps the explanation, assumptions, calculations, and validity checks in the same file.**

## What v0.1 actually does

- handwritten Markdown/directive parser and expression engine
- dependency graph with cycle/unknown-symbol validation
- dependency-ordered computation while preserving document render order
- incremental invalidation and fine-grained DOM patches
- typed inputs, calculations, conditions, assertions, tables, and bar charts
- compiler-style source diagnostics
- static HTML rendering and a hardened loopback-only interactive viewer
- explicit resource limits, fuzzing, race tests, adversarial security tests, and 5,000-node scale benchmarks
- zero third-party Go modules and reproducible release builds

## CLI

```text
md0 validate document.md
md0 eval document.md
md0 render [-o report.html] document.md
md0 open [-addr 127.0.0.1:8080] document.md
md0 inspect document.md
md0 version
```

`render` creates a static HTML snapshot. `open` is intentionally loopback-only. Each browser page gets an isolated reactive session plus a cryptographically random capability token for `/render` requests.

## md0/PURE

> [!IMPORTANT]
> **The document language has no ambient authority.** There is no syntax or evaluator primitive for shell/process execution, arbitrary filesystem access, network requests, environment variables, package imports, native code, arbitrary JavaScript, or dynamic `eval`.

The host runtime may read the document the user explicitly selected and may run the loopback viewer. The **document itself cannot request those capabilities**.

```text
$ md0 inspect examples/parser-benchmark.md

Profile             md0/PURE
...
Dependency graph
  Cycles            0

Evaluation plan
  Mode              dependency-first
  Render order      document order

Document authority
  Filesystem        no
  Network           no
  Shell/processes   no
  Environment       no
  Package imports   no
  Native code       no
  Dynamic eval      no
```

See [`SECURITY.md`](SECURITY.md) for the threat model and local-runtime defenses.

## Language surface

```text
@input  @calc  @show  @when  @assert  @table  @chart
```

Expressions support numeric/string/boolean literals, lists, arithmetic, comparisons, boolean operators, ternaries, and a deliberately small builtin set including `ceil`, `floor`, `round`, `abs`, `sqrt`, `min`, `max`, `len`, `sum`, and `avg`.

There are no general-purpose loops, user-defined functions, modules, imports, arbitrary event handlers, or JavaScript escape hatches.

## Zero dependencies

`go.mod` has no `require` block.

```text
$ go list -m all
github.com/elviscgn/md0
```

CI checks both the module build list **and every package's module ownership** in the complete dependency closure. Go's own `GOROOT/vendor/...` implementation packages remain standard-library packages; they do not become md0 module dependencies.

See [`STDLIB.md`](STDLIB.md) and [`deps-proof.txt`](deps-proof.txt) for the dependency proof.

## Prove it

```bash
make check

go test ./internal/md0 -run='^TestSecurity' -count=1

go test ./internal/md0 -run='^$' -bench='^BenchmarkRuntime$' -benchmem
```

Every push also runs formatting, `go vet`, unit tests, the adversarial security corpus, race detection, parser/evaluator fuzzing, 5,000-node runtime benchmarks, zero-dependency proof, CLI dogfood, byte-identical reproducible builds, and Linux/macOS/Windows portability tests. Workflow actions are pinned to immutable SHAs.

## Evidence

- [`SECURITY.md`](SECURITY.md) — threat model and hardening
- [`PERFORMANCE.md`](PERFORMANCE.md) — measured runtime-scale snapshot and methodology
- [`LIMITS.md`](LIMITS.md) — explicit resource ceilings
- [`STDLIB.md`](STDLIB.md) — standard-library implementation log
- [`DEMO.md`](DEMO.md) — five-minute demonstration path

## Current scope

v0.1 intentionally implements a focused Markdown subset rather than all of CommonMark, and bar charts are the first chart type. The language is deliberately small enough to inspect and bound.

The dependency graph, dependency-first evaluation, incremental recomputation, and targeted DOM patching are implemented today — they are not roadmap claims.

---

<div align="center">

**Too interactive for a static document. Too small, durable, or safety-sensitive to deserve a full application stack.**

</div>
