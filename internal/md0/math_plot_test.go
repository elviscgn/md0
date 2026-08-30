package md0

import (
	"strconv"
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

func TestPlotDirectDocumentValuesAndNamedCurvesAreReactive(t *testing.T) {
	source := `md0: 0.1
Amplitude: @input amplitude number = 2
Frequency: @input frequency number = 1
Phase: @input phase number = 0
Domain: @input domain number = 6
@calc curvature = amplitude / 4

` + "```plot" + `
title = Direct reactive values
wave(x) = amplitude * sin(frequency * x + phase)
quadratic(x) = curvature * pow(x, 2) - 1
x = [-domain, domain]
samples = 96
` + "```"
	doc, err := ParseString("direct-plot.md", source)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewReactiveSession(doc)
	if err != nil {
		t.Fatal(err)
	}
	graph := session.Graph()
	plotNodeID := ""
	for id, node := range graph.Nodes {
		if node.Kind != "markdown" || !containsString(node.DependsOn, "input:frequency") {
			continue
		}
		plotNodeID = id
		for _, dependency := range []string{"input:amplitude", "input:frequency", "input:phase", "input:domain", "calc:curvature"} {
			if !containsString(node.DependsOn, dependency) {
				t.Fatalf("plot dependencies=%v, missing %s", node.DependsOn, dependency)
			}
		}
	}
	if plotNodeID == "" {
		t.Fatalf("named plot dependency node not found: %#v", graph.Nodes)
	}

	fragment, err := RenderFragment(doc, session.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Direct reactive values", "wave(x)", "quadratic(x)", "<svg", "<path"} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("direct named plot missing %q:\n%s", want, fragment)
		}
	}
	if strings.Index(fragment, "wave(x)") >= strings.Index(fragment, "quadratic(x)") {
		t.Fatalf("named curve legend did not preserve declaration order:\n%s", fragment)
	}
	static := RenderStaticPage("Direct plot", fragment)
	if !strings.Contains(static, "<!doctype html>") || !strings.Contains(static, "wave(x)") || !strings.Contains(static, "<svg") {
		t.Fatalf("direct named plot did not survive static rendering:\n%s", static)
	}

	values := session.Values()
	values["amplitude"] = "3"
	result, stats, err := session.Update(values)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(stats.Recomputed, "calc:curvature") || !containsString(stats.Recomputed, plotNodeID) {
		t.Fatalf("direct plot was not invalidated through its graph dependencies: %#v", stats.Recomputed)
	}
	patches, err := RenderPatches(doc, result, stats)
	if err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 || patches[0].NodeID != plotNodeID || !strings.Contains(patches[0].HTML, "wave(x)") {
		t.Fatalf("unexpected direct plot patch: %#v", patches)
	}
}

func TestPlotDependencyDiscoveryCombinesDirectValuesAndInterpolation(t *testing.T) {
	text := "```go\ny = ignored * x\n```\n\n```plot\nf(x) = amplitude * sin(frequency * x) + {{ offset }}\nx = [-domain, domain]\n```"
	deps, err := markdownDependencies(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"amplitude", "domain", "frequency", "offset"} {
		if !containsString(deps, want) {
			t.Fatalf("dependencies=%v, missing %s", deps, want)
		}
	}
	if containsString(deps, "ignored") {
		t.Fatalf("ordinary code fence leaked a plot dependency: %v", deps)
	}
}

func TestPlotNamedCurvesFailClosedOnAmbiguousForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "mixed legacy", src: "f(x) = x\ny2 = -x", want: "cannot be mixed"},
		{name: "empty legacy", src: "f(x) = x\ny =", want: "cannot be mixed"},
		{name: "wrong parameter", src: "f(t) = t", want: "must use x"},
		{name: "reserved name", src: "sin(x) = x", want: "reserved plot name"},
		{name: "legacy label", src: "f(x) = x\nlabel = line", want: "only available with legacy"},
		{name: "duplicate name", src: "f(x) = x\nf(x) = -x", want: "duplicate named curve"},
		{name: "unknown key", src: "colour = red\nf(x) = x", want: "unknown plot key"},
		{name: "bad arity", src: "f(x) = pow(x)", want: "expects 2 arguments"},
		{name: "variadic syntax", src: "f(x) = min(x...)", want: "variadic call syntax"},
		{name: "range local", src: "f(x) = x\nx = [-x, x]", want: "only available inside curve"},
		{name: "too many", src: "a(x)=x\nb(x)=x\nc(x)=x\nd(x)=x\nf(x)=x", want: "at most 4 curves"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := renderPlotFence(test.src)
			if !strings.Contains(got, "md0-plot-error") || !strings.Contains(got, test.want) {
				t.Fatalf("plot did not fail closed with %q:\n%s", test.want, got)
			}
		})
	}
}

func TestSecurityPlotExpressionComplexityIsBounded(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{name: "bytes", expr: strings.Repeat("1+", maxPlotExpressionBytes/2+1) + "1", want: "16 KiB limit"},
		{name: "nodes", expr: "min(" + strings.Repeat("1,", maxPlotExpressionNodes) + "1)", want: "512-node limit"},
		{name: "nesting", expr: strings.Repeat("(", maxPlotExpressionDepth+1) + "x" + strings.Repeat(")", maxPlotExpressionDepth+1), want: "128-level nesting limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := renderPlotFence("f(x) = " + test.expr)
			if !strings.Contains(got, "md0-plot-error") || !strings.Contains(got, test.want) {
				t.Fatalf("plot expression did not enforce %q:\n%s", test.want, got)
			}
		})
	}
}

func TestPlotDirectDocumentValuesMustExistAndBeNumeric(t *testing.T) {
	unknown, err := ParseString("unknown-plot.md", "md0: 0.1\n\n```plot\nf(x) = missing * x\n```")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildDependencyGraph(unknown); err == nil || !strings.Contains(err.Error(), `references unknown value "missing"`) {
		t.Fatalf("unknown direct plot value was not rejected by the graph: %v", err)
	}

	nonNumeric, err := ParseString("string-plot.md", "md0: 0.1\nCaption: @input caption string = \"hello\"\n\n```plot\nf(x) = caption * x\n```")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(nonNumeric, nil); err == nil || !strings.Contains(err.Error(), "must be numeric") || !strings.Contains(err.Error(), "caption") {
		t.Fatalf("non-numeric direct plot value did not produce a clear validation diagnostic: %v", err)
	}
}

func TestPlotDirectDocumentValueMayShareAGoKeywordName(t *testing.T) {
	source := "md0: 0.1\nRange: @input range number = 4\n\n```plot\nf(x) = range * x\nx = [-range, range]\n```"
	doc, err := ParseString("keyword-value-plot.md", source)
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
	if !strings.Contains(fragment, "<svg") || strings.Contains(fragment, "md0-plot-error") {
		t.Fatalf("Go-keyword md0 value did not render as a direct plot dependency:\n%s", fragment)
	}
}

func TestSecurityPlotInterpolationCannotInjectHiddenDirectDependency(t *testing.T) {
	source := "md0: 0.1\nAmplitude: @input amplitude number = 2\n@calc formula = \"amplitude * x\"\n\n```plot\ny = {{ formula }}\n```"
	doc, err := ParseString("hidden-plot-dependency.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(doc, nil); err == nil || !strings.Contains(err.Error(), "unknown document value") || !strings.Contains(err.Error(), "amplitude") {
		t.Fatalf("interpolation introduced a hidden direct plot dependency without a validation error: %v", err)
	}
}

func TestSecurityPlotInterpolationOutputIsBoundedAcrossTheFence(t *testing.T) {
	payload := strings.Repeat("a", maxInputStringBytes/2)
	source := "md0: 0.1\nPayload: @input payload string = " + strconv.Quote(payload) + "\n\n```plot\n" +
		strings.Repeat("# {{ payload }}\n", maxInterpolatedMarkdownBytes/len(payload)+1) +
		"f(x) = x\n```"
	doc, err := ParseString("bounded-plot-interpolation.md", source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Evaluate(doc, nil); err == nil || !strings.Contains(err.Error(), "interpolated Markdown exceeds 4 MiB limit") {
		t.Fatalf("aggregate plot interpolation did not enforce the Markdown output limit: %v", err)
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

func TestSecurityPlotFailsClosedForUnsafeOrUnboundedConfiguration(t *testing.T) {
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
