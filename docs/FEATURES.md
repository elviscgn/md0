# md0 feature reference

This is the practical feature map for md0/PURE 0.1. For the canonical language contract, use [`../SPEC.md`](../SPEC.md).

## Document version

A document may begin with:

```text
md0: 0.1
```

The declaration is metadata and is not rendered. Undeclared documents are interpreted as 0.1 for backward compatibility.

## Typed inputs

Inputs are embedded in prose:

```md
Budget: @input budget currency = 50000
Retries: @input retries integer = 3
Growth: @input growth percent = 12
Enabled: @input enabled boolean = true
Name: @input name string = "baseline"
Timeout: @input timeout duration = 2s
```

Supported types are `number`, `integer`, `percent`, `currency`, `boolean`/`bool`, `string`/`text`, and `duration`.

In the interactive viewer, changing an input invalidates only its affected dependency-graph branch. Inputs render with md0-owned control chrome while retaining semantic HTML input behavior.

## Calculations

```md
@calc projected = base * (1 + growth / 100)
```

Calculations define named values and may depend on inputs, data attachments, or earlier calculations according to dependency order rather than document order.

## Interpolation

```md
Projected revenue is **{{ round(projected) }}**.
```

`{{ expression }}` inserts an evaluated value into Markdown prose. Interpolation does not execute inside ordinary code spans or ordinary fenced code blocks.

`plot` fences are intentionally semantic and may use interpolation to bind md0 values into a graph formula.

## Explicit value display

```md
@show projected
```

`@show` evaluates an expression and renders the result in a code-styled block.

## Conditions

```md
@when projected < minimum
This scenario falls below the approved minimum.
@end
```

Conditional blocks participate in dependency ordering. Definitions created inside a condition cannot leak into an incompatible outer scope.

## Assertions

```md
@assert projected >= minimum
Projected value is below the approved threshold.
@end
```

Assertions do not terminate rendering. They become visible pass/fail document state and are also recorded in snapshots.

## Data attachments

A document declares data without gaining file access:

```md
@data assumptions json
@data measurements csv
```

The operator explicitly binds files:

```bash
md0 open --data assumptions=assumptions.json report.md
md0 render --data measurements=measurements.csv report.md
```

JSON becomes a bounded md0 scalar/list/object value. CSV becomes a bounded table-shaped object. Useful accessors include `get`, `columns`, `rows`, and `column`.

## Tables

```md
@table summary
columns = ["Metric", "Value"]
rows = [["Revenue", revenue], ["Cost", cost], ["Margin", margin]]
@end
```

Tables are evaluated from md0 expressions and react when their dependencies change.

## Bar charts

```md
@chart scenario
labels = ["Revenue", "Cost", "Margin"]
values = [revenue, cost, margin]
@end
```

Version 0.1 `@chart` supports native SVG bar charts. No charting library is loaded.

## Mathematical notation

Inline math uses `$...$`:

```md
Einstein's relation is $E = mc^2$.
```

Display math uses `$$...$$`:

```md
$$
f(x) = \frac{x^2 + 1}{\sqrt{2}}
$$
```

Math is rendered to native browser MathML. It is deliberately a bounded LaTeX-like subset rather than a TeX engine. Reactive values can be inserted normally:

```md
$$
f(x) = {{ amplitude }}\sin({{ frequency }}x)
$$
```

See [`MATH_AND_PLOTS.md`](MATH_AND_PLOTS.md).

## Reactive function plots

A semantic `plot` fence renders a native SVG function graph:

````md
Amplitude: @input amplitude number = 2
Frequency: @input frequency number = 1

```plot
title = Sine family
y = {{ amplitude }} * sin({{ frequency }} * x)
x = [-2*pi, 2*pi]
samples = 320
```
````

Changing `amplitude` or `frequency` causes the Markdown region containing the plot to be recomputed and patched by the existing reactive runtime.

Up to four curves may share one coordinate plane using `y`, `y2`, `y3`, and `y4`, with optional `label`, `label2`, `label3`, and `label4`.

See [`MATH_AND_PLOTS.md`](MATH_AND_PLOTS.md) for the numeric surface, limits, and security model.

## Expression language

General md0 expressions support:

```text
numbers, strings, booleans, lists
identifiers and parentheses
+ - * / %
< <= > >= == !=
&& || !
condition ? yes : no
```

Builtins include:

```text
ceil floor round abs sqrt min max
len sum avg
get columns rows column
```

The plot evaluator has a separate smaller numeric-only allowlist documented in `MATH_AND_PLOTS.md`.

## Static rendering

```bash
md0 render -o report.html report.md
```

Static HTML uses the same shared md0 document design as the interactive viewer, including native MathML and SVG output. It does not include viewer-only settings controls.

## Interactive viewer

```bash
md0 open report.md
```

The viewer runs on loopback only. It provides reactive input updates, targeted DOM patches, source-file reload, runtime diagnostics, export actions, and local presentation preferences.

Viewer preferences include system/light/dark theme, serif/sans/mono typography, density, text size, and page width. Preferences are browser-local presentation state; they do not alter document semantics or saved values.

## Source reload

`md0 open` watches the selected source document. A malformed edit reports a diagnostic while the last valid document remains visible. Once the source becomes valid again, the viewer recovers automatically.

## Save values to Markdown

The interactive viewer can persist the current input values back into the original `@input ... = default` source form while preserving the surrounding document.

## Values files

```bash
md0 open --values values.json report.md
```

A values file is one JSON object containing primitive input values. Snapshot files may also be reused directly as values files.

## Snapshots

```bash
md0 render --snapshot report.snapshot.json -o report.html report.md
```

A snapshot records the input values, source and attachment hashes, md0 language/runtime versions, assertion state, and generated output needed to preserve a durable decision artifact.

## Dependency inspection

```bash
md0 inspect report.md
```

Inspection reports document counts, dependency-graph edges, cycles, evaluation plan, render order, and md0/PURE authority properties.

## Diagnostics

Parser and evaluator errors identify source lines. File-backed CLI diagnostics include source context and a caret when possible.

## Resource limits

md0 enforces explicit document, expression, table, chart, interpolation, output, session, and attachment bounds. Function plots additionally enforce 32–1024 samples and at most four curves. See [`../LIMITS.md`](../LIMITS.md).

## md0/PURE authority boundary

Documents have no syntax or evaluator primitive for arbitrary filesystem access, network requests, shell/process execution, environment access, package imports, native code, arbitrary JavaScript, or dynamic `eval`.

The host runtime may read files the operator explicitly selected and may serve the loopback viewer. The document itself cannot request those capabilities.

See [`../SECURITY.md`](../SECURITY.md).

## Zero third-party dependencies

`go.mod` has no `require` block. The parser, evaluator, dependency graph, Markdown renderer, native MathML renderer, SVG bar charts, SVG function plots, viewer, CSV/JSON loading, HTTP server, hashing, and release tooling are implemented with the Go standard library and repository code.
