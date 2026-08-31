package main

import (
	"strings"
	"testing"
)

func TestEditorHeaderShowsModePathAndSaveState(t *testing.T) {
	editor := testTerminalEditorUX(t, "md0: 0.1\n")
	editor.terminal.color = false
	editor.path = "examples/math-playground.md"

	header := editor.editorHeader(100, defaultTerminalPalette())
	for _, want := range []string{"md0/PURE", "EDIT", "examples/math-playground.md", "✓ saved"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q: %q", want, header)
		}
	}

	editor.dirty = true
	header = editor.editorHeader(100, defaultTerminalPalette())
	if !strings.Contains(header, "● unsaved") {
		t.Fatalf("dirty header=%q", header)
	}
}

func TestEditorShortcutBarUsesEscapeNavigation(t *testing.T) {
	editor := testTerminalEditorUX(t, "")
	editor.terminal.color = false
	bar := editor.editorShortcutBar(100, defaultTerminalPalette(), []editorShortcut{
		{key: "Esc", label: "back"},
		{key: "Ctrl+S", label: "save"},
		{key: "Ctrl+Space", label: "complete"},
		{key: "Tab", label: "indent"},
	})
	for _, want := range []string{"Esc back", "Ctrl+S save", "Ctrl+Space complete", "Tab indent"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("shortcut bar missing %q: %q", want, bar)
		}
	}
	if strings.Contains(bar, "Ctrl+Q") {
		t.Fatalf("shortcut bar should not advertise Ctrl+Q: %q", bar)
	}
}

func TestEditorCompletionShortcutBarPrioritizesDismiss(t *testing.T) {
	editor := testTerminalEditorUX(t, "")
	editor.terminal.color = false
	bar := editor.editorShortcutBar(100, defaultTerminalPalette(), []editorShortcut{
		{key: "↑↓", label: "choose"},
		{key: "Enter/Tab", label: "insert"},
		{key: "Esc", label: "dismiss"},
		{key: "Ctrl+Space", label: "all"},
	})
	for _, want := range []string{"↑↓ choose", "Enter/Tab insert", "Esc dismiss"} {
		if !strings.Contains(bar, want) {
			t.Fatalf("completion bar missing %q: %q", want, bar)
		}
	}
}
