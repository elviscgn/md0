package main

import (
	"bytes"
	"strings"
	"testing"
)

func testTerminalEditorUX(t *testing.T, source string) *terminalEditorUX {
	t.Helper()
	base, _ := testTerminalEditor(t, source)
	base.terminal = terminalUI{out: &bytes.Buffer{}, color: true, palette: defaultTerminalPalette()}
	return &terminalEditorUX{terminalEditor: base}
}

func TestEditorUXGhostCompletionTracksSelectedSuggestion(t *testing.T) {
	editor := testTerminalEditorUX(t, "@w")
	editor.cursorX = len(editor.lines[0])
	editor.refreshCompletionUX(false)
	if !editor.completionOn || len(editor.completion.items) != 1 || editor.completion.items[0].label != "@when" {
		t.Fatalf("completion=%+v", editor.completion)
	}
	if got := editor.completionGhost(); got != "hen " {
		t.Fatalf("ghost=%q, want %q", got, "hen ")
	}
}

func TestEditorUXDirectiveCompletionExpandsBlockSnippet(t *testing.T) {
	editor := testTerminalEditorUX(t, "@wh")
	editor.cursorX = len(editor.lines[0])
	editor.refreshCompletionUX(false)
	editor.acceptCompletionUX()
	if got := editor.source(); got != "@when \n\n@end" {
		t.Fatalf("source=%q", got)
	}
	if editor.cursorY != 0 || editor.cursorX != len([]rune("@when ")) {
		t.Fatalf("cursor=(%d,%d)", editor.cursorY, editor.cursorX)
	}
}

func TestEditorUXPlotFenceCompletionCreatesUsefulScaffold(t *testing.T) {
	editor := testTerminalEditorUX(t, "```p")
	editor.cursorX = len(editor.lines[0])
	editor.refreshCompletionUX(false)
	if !editor.completionOn || editor.completion.items[0].label != "```plot" {
		t.Fatalf("completion=%+v", editor.completion)
	}
	editor.acceptCompletionUX()
	want := "```plot\ny = sin(x)\nx = [-2*pi, 2*pi]\n```"
	if got := editor.source(); got != want {
		t.Fatalf("source=%q, want %q", got, want)
	}
	if editor.cursorY != 1 || editor.cursorX != len([]rune("y = ")) {
		t.Fatalf("cursor=(%d,%d)", editor.cursorY, editor.cursorX)
	}
}

func TestEditorUXBlankPlotLineDoesNotOpenAutomaticMenu(t *testing.T) {
	editor := testTerminalEditorUX(t, "```plot\n\n```")
	editor.cursorY = 1
	editor.cursorX = 0
	items, _ := editor.completionsUX(false)
	if len(items) != 0 {
		t.Fatalf("automatic blank-plot completions=%+v", items)
	}
	items, _ = editor.completionsUX(true)
	if len(items) == 0 {
		t.Fatal("Ctrl+Space should expose plot completions")
	}
}

func TestCompletionPopupOriginPrefersBelowThenFlipsAbove(t *testing.T) {
	row, column := completionPopupOrigin(100, 20, 5, 25, 40, 7)
	if row != 6 || column != 25 {
		t.Fatalf("below origin=(%d,%d)", row, column)
	}
	row, column = completionPopupOrigin(100, 20, 19, 90, 40, 7)
	if row >= 19 {
		t.Fatalf("popup did not flip above cursor: row=%d", row)
	}
	if column != 61 {
		t.Fatalf("right-edge column=%d, want 61", column)
	}
}

func TestEditorUXCompletionPopupIsCursorLocalAndCompact(t *testing.T) {
	editor := testTerminalEditorUX(t, "@")
	editor.cursorX = 1
	editor.refreshCompletionUX(false)
	var out strings.Builder
	editor.drawCompletionPopup(&out, 100, 20, 4, 8, defaultTerminalPalette())
	rendered := out.String()
	if !strings.Contains(rendered, "suggestions 1/") || !strings.Contains(rendered, "@input") {
		t.Fatalf("popup=%q", rendered)
	}
	if strings.Count(rendered, "\x1b[") < 3 {
		t.Fatalf("popup is not positioned as an overlay: %q", rendered)
	}
}
