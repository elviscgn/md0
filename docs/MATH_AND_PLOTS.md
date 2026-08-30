# Math notation and reactive function plots

md0 can typeset mathematical notation and render bounded two-dimensional function plots without KaTeX, MathJax, Plotly, D3, a browser CDN, or any other third-party runtime dependency.

The feature deliberately has two layers:

- mathematical notation is a Markdown rendering feature and becomes native MathML;
- function plots are semantic `plot` fenced blocks and become native SVG.

Both surfaces participate in the existing dependency graph and reactive DOM patching system. Math uses normal md0 interpolation. Plot formulas can refer to numeric document values directly, while retaining interpolation for full md0 expressions.

## Inline math

Use one dollar sign on each side:

```md
The kinetic energy is $E_k = \frac{1}{2}mv^2$.
```

Inline math is not opened when the character after `$` is whitespace, and a closing delimiter must follow non-whitespace. This avoids treating common currency prose such as `Cost is $5 and $10 later` as a formula.

## Display math

Use a display block:

```md
$$
f(x)=\frac{x^2 + 1}{\sqrt{2}}
$$
```

A one-line display is also accepted:

```md
$$ E = mc^2 $$
```

## Reactive notation

Math is rendered after md0 interpolation, so document values can appear inside notation:

```md
Amplitude: @input amplitude number = 2
Frequency: @input frequency number = 1

$$
f(x) = {{ amplitude }}\sin({{ frequency }}x)
$$
```

Changing an input recomputes the Markdown region and therefore the MathML.

## Supported notation surface

The renderer intentionally implements a useful subset rather than a TeX engine.

Supported structure includes:

```text
x^2            superscript
x_1            subscript
x_1^2          combined subscript/superscript
\frac{a}{b}    fraction
\sqrt{x}       square root
\text{...}     text inside math
\left( \right) scalable delimiter markers
```

Common Greek symbols include:

```text
\alpha \beta \gamma \delta \epsilon
\theta \lambda \mu \pi \rho \sigma \phi \omega
\Delta \Sigma \Omega
```

Common operators include:

```text
\cdot \times \div \pm
\le \leq \ge \geq \ne \neq \approx
\infty \sum \prod \int \to
```

Function names such as `\sin`, `\cos`, `\tan`, `\log`, `\ln`, and `\exp` render upright.

Unknown commands are rendered literally rather than gaining macro or package behavior. There is no `\newcommand`, package loading, arbitrary HTML, filesystem access, or TeX execution.

## Function plots

Use a fenced block whose info string is `plot` or `md0-plot`:

````md
```plot
title = Quadratic family
quadratic(x) = 0.5 * pow(x, 2) - 2
x = [-5, 5]
samples = 320
```
````

The block produces a native responsive SVG with grid lines, axes when zero lies inside the visible domain/range, numeric ticks, and one or more curves.

### Reactive parameters

Use existing numeric md0 values directly:

````md
A: @input a number = 1
B: @input b number = 0
C: @input c number = -4

```plot
title = Quadratic explorer
quadratic(x) = a * pow(x, 2) + b * x + c
x = [-8, 8]
samples = 360
```
````

The plot AST collector registers `a`, `b`, and `c` as dependencies before evaluation. Changing any of them invalidates and patches the Markdown region containing the graph. Range bounds may use numeric document values in the same way, for example `x = [-domain, domain]`.

Use `{{ expression }}` when the value needs a full md0 expression rather than a single identifier:

```text
f(x) = {{ amplitude * 2 }} * sin(x)
```

Ordinary fenced code keeps interpolation literal. Only semantic plot fences opt into plot parsing and interpolation.

### Multiple curves

Up to four curves may share one coordinate plane:

````md
```plot
title = Supply and demand
demand(x) = 100 - 1.2 * x
supply(x) = 20 + 0.8 * x
x = [0, 70]
```
````

Each `name(x) = expression` declaration adds a curve in source order, and its name becomes the legend label. The declaration does not create a callable user-defined function. Curve names use the same identifier shape as md0 values, must be unique, and cannot collide with local names, numerical functions, or configuration keys.

Existing documents may continue to use `y`, `y2`, `y3`, and `y4` with optional `label`, `label2`, `label3`, and `label4`. A single fence must use either named curves or legacy curves, not both.

### Plot configuration

At least one named curve or legacy `y` curve is required. Other configuration keys are optional.

| Key | Meaning | Default |
|---|---|---|
| `title` | human-readable plot title | none |
| `name(x)` | preferred named curve declaration | at least one curve required |
| `y`, `y2`–`y4` | legacy curve declarations | at least one curve required |
| `label`, `label2`–`label4` | legacy curve legend labels | expression text |
| `x` | visible numeric domain `[min, max]`; may use document values | `[-10, 10]` |
| `samples` | samples per curve | `320` |

`samples` must be an integer from 32 through 1024. It may be produced with `{{ expression }}`, but unlike curve/range expressions it does not resolve a bare document identifier.

## Plot expression surface

Plot expressions are numeric only. They are parsed with Go's standard-library expression parser and then evaluated by an md0-owned allowlisted AST walker. Parsing an expression does not grant Go execution.

Supported values:

```text
numeric literals
x
pi
e
numeric md0 document values
parentheses
```

Supported arithmetic operators are:

```text
+ - * / %
```

Use `pow(base, exponent)` for exponentiation in plot formulas. Typeset math remains natural LaTeX-like notation, so the displayed equation can use `$x^2$` while the executable plot formula uses `pow(x, 2)`. This keeps formula precedence unambiguous in the current release.

Supported functions:

```text
sin cos tan
asin acos atan
sqrt abs
exp log ln log10
floor ceil round
pow
min max
```

Examples:

```text
sin(x)
2 * cos(3 * x)
pow(x, 2) - 4
pow(x, 3) - x
exp(-pow(x, 2))
log(x)
```

`x` is local to curve expressions and cannot be used in range bounds. `pi`, `e`, and all allowlisted function names are reserved; they cannot be shadowed by document values or curve names. Domain errors such as `sqrt` of a negative value or division by zero create gaps in the sampled curve instead of giving the document extra authority.

## Why direct values remain explicit dependencies

The preferred authoring form reads like ordinary mathematics:

```text
wave(x) = amplitude * sin(frequency * x + phase)
```

This is not ambient environment lookup. Before evaluation, md0 parses each plot expression, distinguishes local/reserved names from external identifiers, and adds every external identifier to the Markdown dependency node. Unknown values fail graph construction. Known values must be numeric and finite.

At render time the evaluator receives only the registered plot values, not the full document environment. Interpolated strings therefore cannot smuggle in a hidden dependency. Interpolation remains useful for inserting the evaluated result of an explicit md0 expression before the same bounded plot parser runs.

## Security properties

The plot evaluator accepts only numeric AST nodes, registered numeric document values, and named functions from its allowlist. It rejects selectors, method calls, indexing, composite literals, strings, unknown/non-numeric identifiers, variadic call syntax, and every other Go AST form not explicitly handled.

For example, `os.Exit(x)` is parsed as syntax but rejected because selector calls are not an allowed plot expression. Nothing is executed.

Plots have explicit CPU/output bounds: at most four curves, at most 1024 samples per curve, and at most 16 KiB / 512 AST nodes / 128 nesting levels per curve or range expression, within md0's existing bounded document and rendered-output limits.

## Static output

MathML and SVG are generated server-side as part of document rendering. Therefore they work in:

```text
md0 open
md0 render
snapshot HTML
```

The generated page does not need a network connection or external JavaScript library to display the equation or graph.

## Flagship example

Run:

```bash
md0 open examples/math-playground.md
```

Tune amplitude, frequency, phase, and curvature and watch both the typeset equation and graph update.
