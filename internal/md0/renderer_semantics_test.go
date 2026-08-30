package md0

import (
	"strings"
	"testing"
)

func TestNegativeChartUsesZeroBaseline(t *testing.T) {
	src := `@chart delta
labels = ["loss", "gain"]
values = [-10, 10]
@end
`
	doc, err := ParseString("negative-chart.md", src)
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
	if !strings.Contains(fragment, `class="bar bar-0 negative"`) {
		t.Fatalf("negative bar was not marked as negative: %s", fragment)
	}
	if !strings.Contains(fragment, `class="axis zero-axis"`) {
		t.Fatalf("chart is missing a semantic zero axis: %s", fragment)
	}
	if strings.Contains(fragment, `y1="210.0" x2="590" y2="210.0" class="axis zero-axis"`) {
		t.Fatalf("mixed-sign chart incorrectly kept zero axis at plot bottom: %s", fragment)
	}
}

func TestPositiveChartKeepsZeroAtBottom(t *testing.T) {
	src := `@chart latency
labels = ["a", "b"]
values = [2, 4]
@end
`
	doc, err := ParseString("positive-chart.md", src)
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
	if !strings.Contains(fragment, `y1="210.0" x2="590" y2="210.0" class="axis zero-axis"`) {
		t.Fatalf("positive-only chart should place zero at plot bottom: %s", fragment)
	}
}

func TestTableRendererProvidesAccessibleStructure(t *testing.T) {
	src := `@table results
columns = ["name", "score"]
rows = [["md0", 42]]
@end
`
	doc, err := ParseString("table-a11y.md", src)
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
	for _, want := range []string{
		`<caption class="md0-sr-only">results</caption>`,
		`<th scope="col">name</th>`,
		`<th scope="col">score</th>`,
		`class="md0-table-scroll"`,
	} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("table output missing %q: %s", want, fragment)
		}
	}
}

func TestInputRendererLinksLabelAndTypeHint(t *testing.T) {
	doc, err := ParseString("input-a11y.md", "Latency: @input latency number = 1.2\n")
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
	inputID := domNodeID("control:latency")
	for _, want := range []string{
		`for="` + inputID + `"`,
		`id="` + inputID + `"`,
		`aria-describedby="` + inputID + `-type"`,
		`id="` + inputID + `-type"`,
	} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("input output missing %q: %s", want, fragment)
		}
	}
}
