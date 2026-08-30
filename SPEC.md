# md0/PURE Language Specification 0.1

This document is the canonical reference for md0 language version `0.1`.

## Version declaration

A document may declare its language version as its first non-empty line:

```text
md0: 0.1
```

The declaration is metadata and is not rendered. Documents without a declaration are interpreted as `0.1` for backward compatibility. A runtime must reject an explicit version it does not support.

## Execution model

An md0 document is UTF-8 Markdown plus md0 directives. The runtime parses the complete document, builds a dependency graph, rejects unknown symbols and cycles, then evaluates nodes in dependency order. Rendering remains in document order.

Inputs can change in an interactive session. Only the changed inputs and their transitive dependents are recomputed. Conditions guard their nested nodes. Assertions report validity but remain part of the rendered document.

## Directives

### Input

```text
Label: @input name type = default_expression
```

Names match `[A-Za-z_][A-Za-z0-9_]*`. Types are `number`, `integer`, `percent`, `currency`, `boolean`/`bool`, `string`/`text`, and `duration`. Duration overrides use Go-style text such as `180ms` or `2s`; their evaluated value is milliseconds.

### Data attachment

```text
@data name json
@data measurements csv
```

The declaration grants no file access. The host must bind each declaration explicitly with `--data name=FILE`. JSON becomes an md0 scalar, list, or object. CSV becomes a bounded table value with a required header row.

### Calculation and display

```text
@calc total = price * quantity
@show total
```

`@calc` defines a value. `@show` renders an evaluated value in a code-styled block. `{{ expression }}` interpolates a value into Markdown prose, except inside code spans and ordinary fenced code blocks. Semantic `plot` fences are the one fenced rendering extension that participates in the dependency graph: they support interpolation and also register bare numeric document values used by plot expressions.

### Condition

```text
@when total > budget
This scenario exceeds budget.
@end
```

The condition must evaluate to a boolean. Definitions inside a condition cannot be referenced outside that conditional scope.

### Assertion

```text
@assert total <= budget
The scenario exceeds the approved budget.
@end
```

The expression must evaluate to a boolean. The body is the failure explanation.

### Table

```text
@table comparison
columns = ["Metric", "Value"]
rows = [["Total", total]]
@end
```

`columns` must be a list. `rows` must be a list of equally sized lists.

### Chart

```text
@chart cost
type = bar
labels = ["Budget", "Total"]
values = [budget, total]
@end
```

Version 0.1 supports bar charts. Labels and values must be non-empty lists of equal length; chart values must be numbers.

## Markdown rendering extensions

Math notation and function plots are bounded rendering extensions layered onto Markdown. They are not new authority-bearing md0 directives and do not expand the document's host capabilities.

### Mathematical notation

Inline mathematical notation uses `$...$` and display notation uses `$$...$$`:

```md
The relation is $E = mc^2$.

$$
f(x)=\frac{x^2 + 1}{\sqrt{2}}
$$
```

The runtime renders a deliberately bounded LaTeX-like subset to native MathML. Supported structure includes superscripts/subscripts, fractions, square roots, text, common Greek symbols, common mathematical operators, and common function names. The renderer is not TeX: it has no macro system, package loading, file inclusion, shell escape, arbitrary HTML, or external asset loading.

Normal md0 interpolation occurs before math rendering, so values may appear inside notation:

```md
$$
f(x)={{ amplitude }}\sin({{ frequency }}x)
$$
```

### Function plots

A fenced block with info string `plot` or `md0-plot` renders a bounded native SVG function plot:

````md
Scale: @input scale number = 0.5

```plot
title = Quadratic family
quadratic(x) = scale * pow(x, 2) - 2
x = [-5, 5]
samples = 320
```
````

Named curves use `name(x) = expression`; the function name becomes the legend label. Bare identifiers in curve expressions and range bounds resolve only to existing numeric md0 values and are registered as reactive dependencies before evaluation. `x`, constants `pi`/`e`, and the numerical function names are reserved. Plot fences may still use `{{ expression }}` when a full md0 expression rather than a single value is needed. Exponentiation uses `pow(base, exponent)` in plot formulas.

Current numerical plot functions are `sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `sqrt`, `abs`, `exp`, `log`, `ln`, `log10`, `floor`, `ceil`, `round`, `pow`, `min`, and `max`.

A plot may contain at most four named curves. The legacy `y`, `y2`, `y3`, `y4` and label keys remain supported, but named and legacy curve forms cannot be mixed in one fence. Each curve uses 32–1,024 samples. The evaluator parses syntax but does not execute Go code; selectors, methods, unknown identifiers, non-numeric document values, strings, indexing, composite literals, and unrecognized expression forms fail closed. `samples` remains a bounded integer literal after optional interpolation. See [`docs/MATH_AND_PLOTS.md`](docs/MATH_AND_PLOTS.md) for the practical plotting reference.

## Values and expressions

Value kinds are null, number, string, boolean, list, and object. Source expressions support numeric, quoted string, boolean, and list literals; identifiers; parentheses; unary `-` and `!`; arithmetic `+ - * / %`; comparisons `< <= > >=`; equality `== !=`; boolean `&& ||`; and `condition ? yes : no`.

Builtins:

- Numeric: `ceil`, `floor`, `round`, `abs`, `sqrt`, `min`, `max`
- Collections: `len`, `sum`, `avg`
- Attached data: `get(object, key)`, `columns(csv)`, `rows(csv)`, `column(csv, name)`

There are no loops, user-defined functions, imports, mutation, dynamic evaluation, or arbitrary event handlers.

## Host-provided values

`--values FILE` accepts one JSON object whose values are strings, numbers, or booleans. An `md0.snapshot/v1` file can also be used directly as a values file. Unknown input names and type mismatches are errors.

`--data NAME=FILE` is repeatable. Every binding must match an `@data` declaration, every declaration must be bound for evaluation, and a document cannot construct a path itself.

## Diagnostics

Parse and evaluation errors identify the source line. File-backed CLI diagnostics include the path, line, inferred column, source text, and a caret where possible. The live viewer keeps the last valid document visible while showing diagnostics for malformed edits.

## Limits and authority

All implementations must enforce finite bounds appropriate to the limits documented in [`LIMITS.md`](LIMITS.md). The md0/PURE authority boundary is defined in [`SECURITY.md`](SECURITY.md): document syntax cannot access arbitrary files, the network, processes, environment variables, packages, native code, or dynamic evaluation.
