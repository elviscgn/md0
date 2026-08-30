# md0 documentation

This directory is the practical manual for humans and AI agents working with md0.

md0 is a zero-third-party-dependency runtime for interactive Markdown that remains a document. The document may declare inputs, calculations, conditions, assertions, tables, charts, mathematical notation, and bounded function plots, but it has no ambient authority to execute shell commands, access arbitrary files, use the network, import packages, or run arbitrary JavaScript.

## Where to start

- [`FEATURES.md`](FEATURES.md) — user-facing reference for every supported feature with examples.
- [`MATH_AND_PLOTS.md`](MATH_AND_PLOTS.md) — mathematical notation and reactive function plotting.
- [`CLI_AND_EDITOR.md`](CLI_AND_EDITOR.md) — document launcher, terminal presentation, and built-in live authoring mode.
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — parser → AST → dependency graph → evaluation → rendering → reactive updates.
- [`AGENTS.md`](AGENTS.md) — repository guidance for AI coding agents and automated contributors.
- [`../SPEC.md`](../SPEC.md) — canonical md0/PURE 0.1 language contract.
- [`../SECURITY.md`](../SECURITY.md) — authority boundary and runtime threat model.
- [`../LIMITS.md`](../LIMITS.md) — resource ceilings.
- [`../STDLIB.md`](../STDLIB.md) — standard-library implementation and zero-dependency evidence.
- [`../PERFORMANCE.md`](../PERFORMANCE.md) — runtime-scale measurements and methodology.
- [`../DEMO.md`](../DEMO.md) — short demonstration path.

## Source-of-truth order

When documentation disagrees, use this order:

1. `SPEC.md` for language semantics and syntax.
2. `SECURITY.md` and `LIMITS.md` for authority and resource boundaries.
3. tests in `internal/md0` for executable behavior and edge cases.
4. this `docs/` directory for practical usage and contributor guidance.
5. `README.md` for the public overview.

Math notation and `plot` fences are rendering features layered onto Markdown rather than new md0/PURE directives. Their current contract is documented in `MATH_AND_PLOTS.md` and covered by executable tests.

The CLI launcher and built-in editor are host tooling, not new document-language authority. Their behavior and save boundary are documented in `CLI_AND_EDITOR.md`.

## Flagship examples

- [`../examples/parser-benchmark.md`](../examples/parser-benchmark.md) — reactive benchmark comparison, chart, assertion, and dependency graph.
- [`../examples/scenario-model.md`](../examples/scenario-model.md) — financial scenario with attached JSON data.
- [`../examples/incident-report.md`](../examples/incident-report.md) — CSV-backed reliability report.
- [`../examples/decision-record.md`](../examples/decision-record.md) — interactive engineering decision record.
- [`../examples/math-playground.md`](../examples/math-playground.md) — reactive MathML equations and SVG function plots.

## Design rule

A feature belongs in md0 when it strengthens documents without quietly turning documents into programs. Prefer bounded declarative syntax, deterministic evaluation, explicit dependencies, native browser output, and Go standard-library implementation over general-purpose escape hatches.
