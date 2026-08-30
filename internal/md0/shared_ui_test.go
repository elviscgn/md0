package md0

import (
	"strings"
	"testing"
)

func TestStaticPageUsesModernDocumentTheme(t *testing.T) {
	page := RenderStaticPage("report.md", `<div class="md0-doc"><h1>Report</h1></div>`)

	for _, want := range []string{
		`--md0-font-serif:Charter`,
		`--paper:#fff`,
		`prefers-color-scheme:dark`,
		`--paper:#17181b`,
		`Avenir Next`,
		`.md0-input{display:grid`,
		`.md0-chart,.md0-table{margin:22px 0;padding:0`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("static page missing modern renderer marker %q", want)
		}
	}

	for _, forbidden := range []string{
		`#fff8f0`,
		`#fffcf8`,
		`Segoe UI`,
		`Inter,`,
		`"Inter"`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("static page still contains legacy renderer marker %q", forbidden)
		}
	}
}
