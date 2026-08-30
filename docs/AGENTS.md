# AI agent guide for md0

This file is written for coding agents, automated reviewers, and humans using AI-assisted development.

The goal is to let an agent modify md0 without first reverse-engineering its safety model or accidentally turning a bounded document runtime into a general-purpose code execution system.

## Read this first

Before changing behavior, read:

1. `SPEC.md` — canonical md0/PURE 0.1 language semantics.
2. `SECURITY.md` — document authority and loopback viewer threat model.
3. `LIMITS.md` — explicit resource ceilings.
4. `docs/FEATURES.md` — practical feature surface and examples.
5. `docs/ARCHITECTURE.md` — runtime/data-flow map.
6. relevant tests under `internal/md0`.

For math/plot changes, also read `docs/MATH_AND_PLOTS.md` and `internal/md0/math_plot_test.go`.

## Non-negotiable invariants

- Keep `go.mod` free of third-party `require` directives.
- Do not add package-manager, CDN, or runtime network dependencies.
- Do not add shell/process execution to document syntax.
- Do not add arbitrary filesystem access to document syntax.
- Do not expose environment variables to documents.
- Do not add arbitrary JavaScript or dynamic `eval` escape hatches.
- Keep rendering bounded and deterministic.
- Preserve static HTML functionality; do not make core document rendering depend on the interactive viewer.
- Preserve source readability. An md0 file should remain understandable as a document without running it.

## Change strategy

Prefer the smallest layer that can express a feature.

Examples:

- presentation-only change → shared document styling/rendering;
- Markdown notation change → Markdown renderer;
- new reactive value definition → AST/parser/graph/evaluator;
- new visualization that only consumes interpolated values → consider a semantic Markdown rendering extension before expanding the core AST;
- new data capability → keep path/file selection on the host side.

Do not create a new directive merely because it is convenient if an existing bounded layer can express the same behavior cleanly.

## Dependency graph rules

Every value dependency must be visible to the graph before evaluation.

For core AST nodes, use `ExprDependencies` and add the dependency node in `BuildDependencyGraph`.

For Markdown rendering, use normal `{{ expression }}` interpolation. Ordinary code spans and ordinary code fences intentionally suppress interpolation.

`plot` fences are semantic and intentionally allow `{{ expression }}` so plotted parameters are explicit graph dependencies.

Never hide document-variable lookup inside a renderer without also making that dependency visible to invalidation.

## Rendering rules

Generated HTML must escape user-controlled text unless the renderer itself constructs the markup from a bounded parsed representation.

Prefer native browser primitives:

- semantic HTML inputs;
- SVG for charts/plots;
- MathML for mathematics;
- CSS variables from the shared md0 document theme.

Do not solve presentation by injecting a third-party client library.

## Math and plot rules

Math notation is a bounded LaTeX-like renderer, not TeX. Do not add macros, package loading, arbitrary HTML commands, filesystem primitives, or command execution.

Plot expressions are parsed with Go's standard-library parser but are never compiled or executed as Go. Only AST node kinds explicitly handled by the numeric evaluator are allowed.

Keep plot limits bounded. Current limits are at most four curves and 32–1024 samples per curve.

If adding a plot function, add explicit evaluator handling and tests for its domain/non-finite behavior.

## Security review checklist

For any new syntax or builtin, answer these questions in the PR:

- What values can the document read?
- What host resources can it cause to be touched?
- Is evaluation bounded by input/document limits?
- Can user text become unescaped HTML/JS?
- Can the feature make network requests?
- Can it open files or construct paths?
- Can it spawn processes?
- Can it load packages or native code?
- Does it introduce hidden dependencies that reactive invalidation cannot see?

If any answer expands authority, update `SECURITY.md` and treat the change as architectural, not cosmetic.

## Testing expectations

At minimum, feature changes should include:

- happy-path behavior;
- malformed syntax;
- boundary/resource limit behavior;
- security/fail-closed behavior where applicable;
- reactive dependency behavior for anything that consumes inputs/calculations;
- static rendering if the feature appears in HTML;
- preservation of ordinary Markdown/code behavior around the extension.

Run the repository's full CI before merge. The expected verification includes formatting, vet, unit tests, adversarial security tests, race detection, fuzzing, scale benchmarks, zero-dependency proof, builds, CLI smoke tests, portability, and reproducibility.

## Documentation expectation

When behavior changes, update the smallest canonical docs that make the behavior discoverable:

- `SPEC.md` for language syntax/semantics;
- `SECURITY.md` for authority changes;
- `LIMITS.md` for resource limits;
- `docs/FEATURES.md` for user-visible capabilities;
- a focused guide under `docs/` for substantial subsystems;
- `examples/` for runnable demonstrations.

Do not rewrite the public README from scratch for a narrow feature. Keep it an overview and link into detailed docs.

## Useful repository areas

```text
cmd/md0/                 CLI entry point
internal/md0/parser.go   document parser
internal/md0/ast.go      AST nodes
internal/md0/expr.go     md0 expression engine
internal/md0/graph.go    dependency graph
internal/md0/plan.go     dependency-first evaluation plan
internal/md0/eval.go     evaluator
internal/md0/reactive.go incremental session state
internal/md0/markdown.go Markdown renderer
internal/md0/interpolation.go reactive Markdown interpolation
internal/md0/math_plot.go native MathML + bounded SVG function plots
internal/md0/render.go   node/HTML rendering
internal/md0/browser.go  loopback interactive viewer
internal/md0/document_style.go shared document visual system
examples/                runnable md0 documents
docs/                    human + agent manual
```

## Definition of done

A change is not done because it works once in the browser. It is done when its semantics are explicit, authority stays bounded, static and interactive behavior are coherent, regression tests exist, documentation/examples are updated, and the full CI suite passes on the exact commit intended for merge.
