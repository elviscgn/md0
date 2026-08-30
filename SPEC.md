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

`@calc` defines a value. `@show` renders an evaluated value in a code-styled block. `{{ expression }}` interpolates a value into Markdown prose, except inside code spans and fenced code blocks.

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
