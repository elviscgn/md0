# md0 architecture

md0 is intentionally small enough that the complete execution model can be understood without a framework stack.

## Pipeline

```text
Markdown source
  → parser
  → AST
  → dependency graph
  → dependency-first evaluation plan
  → evaluated environment + assertion/condition state
  → document-order renderer
  → static HTML or interactive viewer
  → incremental affected-node patches after input updates
```

Document order controls presentation. Dependency order controls computation.

## Parser

Primary implementation: `internal/md0/parser.go`.

The parser recognizes the md0 0.1 declaration, prose Markdown, `@input`, `@data`, `@calc`, `@show`, `@when`, `@assert`, `@table`, and `@chart`.

Fenced Markdown is preserved as Markdown content. The Markdown renderer later recognizes `plot`/`md0-plot` as one semantic fenced rendering extension.

## AST

Primary implementation: `internal/md0/ast.go`.

The AST keeps declarative document constructs as small typed nodes. Calculation and condition expressions use the bounded md0 expression AST.

Mathematical notation and plot fences intentionally remain inside `MarkdownNode` for the current vertical slice. This lets them inherit Markdown interpolation/reactivity without expanding the core language AST or authority model.

## Expressions

Primary implementation: `internal/md0/expr.go`.

The md0 expression engine is handwritten and deterministic. It evaluates literals, identifiers, lists, arithmetic, comparisons, boolean logic, ternaries, and a small builtin set.

The function-plot evaluator is separate and smaller. `internal/md0/math_plot.go` uses Go's standard `go/parser` only to parse numeric expression syntax, then walks the AST itself with an explicit allowlist. It does not execute Go code.

## Dependency graph

Primary implementation: `internal/md0/graph.go`.

Each reactive/semantic node receives a stable dependency node ID. Producers define named values. Consumers record symbolic dependencies. Unknown values and cycles are rejected before evaluation.

Markdown dependencies come from `{{ expression }}` interpolation. Ordinary code spans and ordinary fenced code suppress interpolation. Semantic plot fences opt into interpolation so graph parameters are explicit in source and automatically participate in invalidation.

## Evaluation plan

Primary implementation: `internal/md0/plan.go` and `internal/md0/eval.go`.

The dependency graph is converted into deterministic dependency-first order. Conditional guards are tracked so definitions cannot escape invalid scopes.

Input updates reuse the plan and recompute only affected values/nodes.

## Reactive sessions

Primary implementation: `internal/md0/reactive.go` and the browser runtime in `internal/md0/browser.go`.

A session distinguishes explicit user overrides from values merely displayed by dependent inputs. After an update, md0 computes the affected graph branch and generates targeted DOM patches rather than rebuilding the entire browser page.

## Markdown rendering

Primary implementation: `internal/md0/markdown.go` and `internal/md0/interpolation.go`.

The renderer intentionally implements a focused Markdown subset. Code spans/fences are escaped. Strong text, headings, lists, and prose are rendered directly.

Inline/display math is converted to native MathML by `internal/md0/math_plot.go`.

`plot` fences are converted to bounded native SVG by the same file.

## Document rendering

Primary implementation: `internal/md0/render.go`.

Evaluated AST nodes are rendered in document order. Renderable regions receive stable DOM identifiers, allowing the interactive runtime to replace only affected regions.

Bar charts and function plots are generated as SVG rather than using a chart library.

## Shared presentation

Primary implementation: `internal/md0/document_style.go`.

Static HTML, snapshot HTML, and the interactive document consume the same document visual language. Viewer-only controls remain in `browser.go` so exported documents do not depend on the viewer shell.

## Static output and snapshots

`md0 render` creates self-contained HTML. Optional snapshots record values and provenance together with generated output.

MathML and SVG are rendered into that same output, so math/plot documents do not require network access after rendering.

## Explicit data attachments

Primary implementation: attachment/data loading files under `internal/md0`.

A document may declare data by name and format, but only the host/operator can bind a path. The evaluator receives already-loaded bounded data values; document syntax cannot discover paths.

## Security boundary

The key architectural distinction is host authority versus document authority.

The md0 executable may read the source/data files explicitly selected by the operator and may open a loopback HTTP listener. The document language itself has no primitives for filesystem discovery, network I/O, processes, environment variables, packages, native code, arbitrary JavaScript, or dynamic evaluation.

See `SECURITY.md` for the full threat model.

## Tests and verification

Tests live primarily under `internal/md0`. CI performs formatting, vet, unit tests, adversarial security tests, race detection, fuzzing, 5,000-node scale benchmarks, dependency proof, build/smoke tests, portability checks, and reproducible-build verification.

Feature work should preserve this property: if a new feature cannot be bounded, tested, and represented clearly in the authority model, it should not be merged into md0/PURE.
