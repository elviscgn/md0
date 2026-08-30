package md0

import (
	"strings"
	"testing"
)

func TestMathRenderingUsesNativeMathML(t *testing.T) {
	got := renderMarkdown(`Inline $x^2 + \alpha$ math.

$$
f(x)=\frac{x^2+1}{\sqrt{2}}
$$`)
	for _, want := range []string{"<math", "<msup>", "<mfrac>", "<msqrt>", "α"} {
		if !strings.Contains(got, want) {
			t.Fatalf("math rendering missing %q:\n%s", want, got)
		}
	}
}

func TestMathRenderingEscapesMarkup(t *testing.T) {
	got := renderMarkdown(`$x < script > y$`)
	if strings.Contains(strings.ToLower(got), "<script") {
		t.Fatalf("math renderer emitted executable markup: %s", got)
	}
	if !strings.Contains(got, "&lt;") || !strings.Contains(got, "&gt;") {
		t.Fatalf("math renderer did not escape operators safely: %s", got)
	}
}

func TestInlineMathDoesNotTreatOrdinaryCurrencyAsFormula(t *testing.T) {
	got := renderInline("Cost is $5 and $10 later")
	if strings.Contains(got, "<math") {
		t.Fatalf("currency text was treated as inline math: %s", got)
	}
}

func TestPlotFenceInterpolationsAreReactiveButCodeFencesStayLiteral(t *testing.T) {
	text := "```go\n{{ ignored }}\n```\n\n```plot\ny = {{ amplitude }} * sin(x)\nx = [-pi, pi]\n```"
	deps, err := markdownInterpolationDependencies(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0] != "amplitude" {
		t.Fatalf("plot dependencies=%v, want [amplitude]", deps)
	}
	got, err := interpolateMarkdown(text, map[string]Value{"amplitude": Number(3)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "{{ ignored }}") {
		t.Fatalf("ordinary code fence interpolation changed:\n%s", got)
	}
	if !strings.Contains(got, "y = 3 * sin(x)") {
		t.Fatalf("plot interpolation did not update:\n%s", got)
	}
}

func TestReactivePlotChangesWithInput(t *testing.T) {
	source := "md0: 0.1\nAmplitude: @input amplitude number = 2\n\nThe family is $f(x) = {{ amplitude }} \\sin(x)$.\n\n```plot\ntitle = Reactive sine wave\ny = {{ amplitude }} * sin(x)\nx = [-pi, pi]\nsamples = 96\n```"
	doc, err := ParseString("math.md", source)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstHTML, err := RenderFragment(doc, first)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<math", "<svg", "<path", "Reactive sine wave"} {
		if !strings.Contains(firstHTML, want) {
			t.Fatalf("initial render missing %q:\n%s", want, firstHTML)
		}
	}
	second, err := Evaluate(doc, map[string]string{"amplitude": "3"})
	if err != nil {
		t.Fatal(err)
	}
	secondHTML, err := RenderFragment(doc, second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHTML == secondHTML {
		t.Fatal("plot and math markup did not change after reactive input update")
	}
	if !strings.Contains(secondHTML, "<mn>3</mn>") {
		t.Fatalf("reactive MathML did not receive updated value:\n%s", secondHTML)
	}
}

func TestPlotSupportsMultipleCurvesAndPow(t *testing.T) {
	got := renderPlotFence("title = Functions\ny = pow(x, 2)\nlabel = quadratic\ny2 = sin(x)\nlabel2 = sine\nx = [-3, 3]\nsamples = 64")
	for _, want := range []string{"quadratic", "sine", "var(--chart-0)", "var(--chart-1)", "<path"} {
		if !strings.Contains(got, want) {
			t.Fatalf("multi-curve plot missing %q:\n%s", want, got)
		}
	}
}

func TestPlotEscapesTitleAndLabels(t *testing.T) {
	got := renderPlotFence("title = <script>alert(1)</script>\ny = x\nlabel = <img src=x onerror=alert(1)>\ny2 = -x\nlabel2 = safe\nx = [-1, 1]")
	lower := strings.ToLower(got)
	for _, unsafe := range []string{"<script", "<img"} {
		if strings.Contains(lower, unsafe) {
			t.Fatalf("plot emitted unsafe markup %q: %s", unsafe, got)
		}
	}
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&lt;img src=x onerror=alert(1)&gt;") {
		t.Fatalf("plot title/label were not escaped: %s", got)
	}
}

func TestPlotFailsClosedForUnsafeOrUnboundedConfiguration(t *testing.T) {
	unsafe := renderPlotFence("y = os.Exit(x)")
	if !strings.Contains(unsafe, "md0-plot-error") {
		t.Fatalf("unsafe selector was not rejected: %s", unsafe)
	}
	oversampled := renderPlotFence("y = x\nsamples = 1025")
	if !strings.Contains(oversampled, "samples must be an integer from 32 to 1024") {
		t.Fatalf("oversampled plot was not rejected: %s", oversampled)
	}
}

func TestMathAndPlotSurviveStaticPageRendering(t *testing.T) {
	source := "md0: 0.1\nScale: @input scale number = 2\n\n$$f(x) = {{ scale }}x^2$$\n\n```plot\ny = {{ scale }} * pow(x, 2)\nx = [-2, 2]\nsamples = 64\n```"
	doc, err := ParseString("static-math.md", source)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Evaluate(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := RenderFragment(doc, result)
	if err != nil {
		t.Fatal(err)
	}
	page := RenderStaticPage("Static math", fragment)
	for _, want := range []string{"<!doctype html>", "<math", "<msup>", "<svg", "<path"} {
		if !strings.Contains(page, want) {
			t.Fatalf("static page missing %q", want)
		}
	}
	for _, forbidden := range []string{"katex", "mathjax", "plotly", "d3.js"} {
		if strings.Contains(strings.ToLower(page), forbidden) {
			t.Fatalf("static page unexpectedly references %q", forbidden)
		}
	}
}
