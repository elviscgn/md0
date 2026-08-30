package md0

import (
	"strings"
	"testing"
)

func TestViewerCSSDoesNotRedefineDocumentTheme(t *testing.T) {
	for _, forbidden := range []string{
		`--paper:`,
		`--md0-font-serif:`,
		`.md0-doc h1`,
		`.md0-chart,.md0-table`,
		`.md0-input{`,
	} {
		if strings.Contains(runtimeStatusCSS, forbidden) {
			t.Fatalf("viewer CSS must not redefine shared document styling %q", forbidden)
		}
	}
}

func TestAllDocumentSurfacesUseSharedCSS(t *testing.T) {
	fragment := `<div class="md0-doc"><h1>Shared theme</h1></div>`
	staticPage := RenderStaticPage("static.md", fragment)
	interactivePage := RenderInteractivePage("interactive.md", fragment)

	for name, page := range map[string]string{
		"static":      staticPage,
		"interactive": interactivePage,
	} {
		if !strings.Contains(page, documentCSS) {
			t.Fatalf("%s page does not include shared document CSS", name)
		}
		if strings.Contains(page, "#fff8f0") || strings.Contains(page, "#fffcf8") {
			t.Fatalf("%s page contains legacy beige theme", name)
		}
	}
}
