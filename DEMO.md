# md0 — 5 minute demo

## 0:00 — The problem

Open `examples/parser-benchmark.md` as plain text.

> Benchmarks are already documents. The problem is that their conclusions become stale when the numbers change.

md0 keeps the inputs, calculations, prose, charts, and validity checks in the same reviewable Markdown file.

## 0:35 — Build and validate

```bash
make build
./bin/md0 version
./bin/md0 validate examples/parser-benchmark.md
```

Expected version:

```text
md0 v0.1.0
```

Then show that `go.mod` contains no `require` block:

```bash
go list -m all
```

Only the md0 module should be listed.

## 1:15 — Show the runtime plan

```bash
./bin/md0 inspect examples/parser-benchmark.md
```

Point out three things:

1. profile is `md0/PURE`
2. the dependency graph reports zero cycles
3. computation is dependency-first while render order remains document order

Then show the authority matrix: filesystem, network, shell/processes, environment, package imports, native code, and dynamic eval are all unavailable to the document language.

## 2:05 — Make the document live

```bash
./bin/md0 open examples/parser-benchmark.md
```

In the browser, change `candidate_ms` from `1.31` to `2.10`.

Show the visible consequence:

- derived prose changes from improvement to regression
- the chart moves
- the regression assertion fails
- unrelated DOM regions are not replaced

Explain that md0 uses the dependency graph to recompute and patch only affected regions.

Edit the source file while the viewer stays open. A valid change reloads automatically; a malformed expression shows a source diagnostic while the last valid page remains visible. Fix the expression and show automatic recovery.

Use **Export snapshot** to download the current values, source hash, versions, assertions, and generated HTML. Use **Save values to Markdown** to create a readable document with the explored values written back as defaults.

## 3:10 — Security boundary

The live viewer is intentionally local:

- it refuses non-loopback bind addresses
- each page load gets an isolated reactive session
- each page gets a fresh 256-bit capability token required by `/render`
- hostile Host / Origin / cross-site requests are rejected
- rendered document data is HTML-escaped
- CSP allows the built-in scripts/styles only by exact SHA-256 hashes
- request, string-input, interpolation, render-output, nesting, table, and chart work are bounded

Show the proof target:

```bash
go test ./internal/md0 -run='^TestSecurity' -count=1
```

Do not describe md0 as formally audited; the claim is that these boundaries are explicit and regression-tested for the stated md0/PURE threat model.

## 4:00 — Performance and dogfooding

Open `PERFORMANCE.md` and `examples/runtime-scale.md`.

The CI benchmark harness measures parse, planning, evaluation, rendering, and incremental updates on synthetic 100 / 1,000 / 5,000-node chain and fan-out workloads.

The runtime-scale report is itself an md0 computational document built from measured benchmark data.

## 4:35 — Close

Show the original `.md` file again.

No framework project appeared. No package graph appeared. The source remained readable Markdown.

> **Documents should be able to compute without becoming software.**

Final line:

> **md0: interactivity without authority.**
