package md0

import (
	"strings"
	"testing"
)

func TestInteractiveRuntimePageIncludesViewerPreferences(t *testing.T) {
	page := renderInteractiveRuntimePageRevision("report.md", `<div class="md0-doc">report</div>`, "token", "revision")

	for _, want := range []string{
		`id="md0-settings-toggle"`,
		`id="md0-settings-panel"`,
		`data-md0-preference="theme"`,
		`data-md0-preference="font"`,
		`data-md0-preference="density"`,
		`data-md0-preference="textSize"`,
		`data-md0-preference="width"`,
		`md0:preferences:v1`,
		`>System<`,
		`>Light<`,
		`>Dark<`,
		`md0-segmented`,
		`md0EnhanceInputs`,
		`md0-stepper`,
		`Avenir Next`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("interactive runtime page missing viewer preference marker %q", want)
		}
	}
	if strings.Contains(page, "Inter,") || strings.Contains(page, `"Inter"`) {
		t.Fatal("interactive runtime page unexpectedly includes Inter in its font stack")
	}
}

func TestStaticPageDoesNotIncludeViewerControls(t *testing.T) {
	page := RenderStaticPage("report.md", `<div class="md0-doc">report</div>`)
	for _, forbidden := range []string{
		`md0-settings-toggle`,
		`md0-settings-panel`,
		`md0:preferences:v1`,
		`md0-export-snapshot`,
	} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("static page unexpectedly contains viewer-only control %q", forbidden)
		}
	}
}
