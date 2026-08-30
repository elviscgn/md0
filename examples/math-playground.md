md0: 0.1

# Reactive Math Playground

This document keeps the explanation, parameters, typeset mathematics, and graphs in one readable Markdown file.

Amplitude: @input amplitude number = 2
Frequency: @input frequency number = 1
Phase: @input phase number = 0
Curvature: @input curvature number = 0.25

## Wave family

The current function is

$$
f(x) = {{ amplitude }}\sin({{ frequency }}x + {{ phase }})
$$

Change amplitude, frequency, or phase above. The equation and SVG curve update from the same md0 dependency graph.

```plot
title = Reactive wave and quadratic
y = {{ amplitude }} * sin({{ frequency }} * x + {{ phase }})
label = wave
y2 = {{ curvature }} * pow(x, 2) - 1
label2 = quadratic
x = [-2*pi, 2*pi]
samples = 420
```

## Math notation

Inline math works inside prose, for example $E = mc^2$ and $\alpha + \beta = \gamma$.

Display math supports a deliberately bounded LaTeX-like surface and renders with native MathML:

$$
\frac{x^2 + 1}{\sqrt{2}} \ge 0
$$

The plot evaluator is numeric and bounded. It supports arithmetic, `x`, constants `pi` and `e`, and allowlisted functions such as `sin`, `cos`, `tan`, `sqrt`, `log`, `exp`, `abs`, and `pow`.
