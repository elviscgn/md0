# Math notation and reactive function plots

md0 can typeset mathematical notation and render bounded two-dimensional function plots without KaTeX, MathJax, Plotly, D3, a browser CDN, or any other third-party runtime dependency.

The feature deliberately has two layers:

- mathematical notation is a Markdown rendering feature and becomes native MathML;
- function plots are semantic `plot` fenced blocks and become native SVG.

Both surfaces can use normal md0 interpolation so input changes flow through the existing dependency graph and reactive DOM patching system.

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
y = 0.5 * x^2 - 2
x = [-5, 5]
samples = 320
```
````

The block produces a native responsive SVG with grid lines, axes when zero lies inside the visible domain/range, numeric ticks, and one or more curves.

### Reactive parameters

Use ordinary md0 interpolation for document dependencies:

````md
A: @input a number = 1
B: @input b number = 0
C: @input c number = -4

```plot
title = Quadratic explorer
y = {{ a }} * x^2 + {{ b }} * x + {{ c }}
x = [-8, 8]
samples = 360
```
````

The `{{ a }}`, `{{ b }}`, and `{{ c }}` expressions are tracked by the same dependency graph used for reactive prose. Ordinary fenced code continues to keep interpolation literal; only semantic plot fences opt into interpolation.

### Multiple curves

Up to four curves may share one coordinate plane:

````md
```plot
title = Supply and demand
y = 100 - 1.2 * x
label = demand
y2 = 20 + 0.8 * x
label2 = supply
x = [0, 70]
```
````

Use `y`, `y2`, `y3`, and `y4`. Optional labels use `label`, `label2`, `label3`, and `label4`.

### Plot configuration

`y` is required. Other keys are optional.

| Key | Meaning | Default |
|---|---|---|
| `title` | human-readable plot title | none |
| `y` | first function of `x` | required |
| `y2`–`y4` | additional functions | none |
| `label`–`label4` | curve legend labels | expression text |
| `x` | visible domain `[min, max]` | `[-10, 10]` |
| `samples` | samples per curve | `320` |

`samples` must be an integer from 32 through 1024.

## Plot expression surface

Plot expressions are numeric only. They are parsed with Go's standard-library expression parser and then evaluated by an md0-owned allowlisted AST walker. Parsing an expression does not grant Go execution.

Supported values:

```text
numeric literals
x
pi
e
parentheses
```

Supported operators:

```text
+ - * / %
^
```

Inside a plot, `^` is intentionally interpreted as exponentiation, so `x^2` means x squared.

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
x^2 - 4
pow(x, 3) - x
exp(-x^2)
log(x)
```

Domain errors such as `sqrt` of a negative value or division by zero create gaps in the sampled curve instead of giving the document extra authority.

## Why interpolation is explicit in plots

Plot formulas use `{{ value }}` for md0 document parameters rather than silently resolving arbitrary names from the document environment:

```text
y = {{ amplitude }} * sin({{ frequency }} * x)
```

This keeps dependencies visible in the source, reuses the existing interpolation graph, and gives the plot evaluator a tiny local namespace: `x`, `pi`, `e`, numeric functions, and interpolated numeric constants.

If a bare unknown name remains, plotting fails closed with guidance to use interpolation.

## Security properties

The plot evaluator accepts only numeric AST nodes and named functions from its allowlist. It rejects selectors, method calls, indexing, composite literals, strings, arbitrary identifiers, and every other Go AST form not explicitly handled.

For example, `os.Exit(x)` is parsed as syntax but rejected because selector calls are not an allowed plot expression. Nothing is executed.

Plots have explicit CPU/output bounds: at most four curves and at most 1024 samples per curve, within md0's existing bounded document and rendered-output limits.

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
