package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testTerminalEditor(t *testing.T, source string) (*terminalEditor, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "document.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return newTerminalEditor(path, source, terminalUI{out: &bytes.Buffer{}, color: true, palette: defaultTerminalPalette()}), path
}

func TestTerminalEditorEditsAndSavesInPlace(t *testing.T) {
	editor, path := testTerminalEditor(t, "md0: 0.1\n# Draft\n")
	editor.cursorY, editor.cursorX = 1, len(editor.lines[1])
	if quit, err := editor.handle(editorEvent{key: editorKeyRune, r: '!'}); quit || err != nil {
		t.Fatalf("insert quit=%v err=%v", quit, err)
	}
	if !editor.dirty || editor.source() != "md0: 0.1\n# Draft!\n" {
		t.Fatalf("source=%q dirty=%v", editor.source(), editor.dirty)
	}
	if quit, err := editor.handle(editorEvent{key: editorKeySave}); quit || err != nil {
		t.Fatalf("save quit=%v err=%v", quit, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != editor.source() || editor.dirty {
		t.Fatalf("disk=%q source=%q dirty=%v", got, editor.source(), editor.dirty)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed to %o", info.Mode().Perm())
	}
}

func TestTerminalEditorPreservesCRLFLineEndings(t *testing.T) {
	editor, path := testTerminalEditor(t, "md0: 0.1\r\n# Draft\r\n")
	if got := editor.source(); got != "md0: 0.1\r\n# Draft\r\n" {
		t.Fatalf("source line endings=%q", got)
	}
	editor.cursorY, editor.cursorX = 1, len(editor.lines[1])
	_, _ = editor.handle(editorEvent{key: editorKeyRune, r: '!'})
	if err := editor.save(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "md0: 0.1\r\n# Draft!\r\n" {
		t.Fatalf("saved line endings=%q", got)
	}
}

func TestTerminalEditorRejectsStaleSave(t *testing.T) {
	editor, path := testTerminalEditor(t, "md0: 0.1\n# Original\n")
	editor.cursorY, editor.cursorX = 1, len(editor.lines[1])
	_, _ = editor.handle(editorEvent{key: editorKeyRune, r: '!'})
	if err := os.WriteFile(path, []byte("md0: 0.1\n# External\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := editor.save(); err == nil || !strings.Contains(err.Error(), "source changed on disk") {
		t.Fatalf("stale save error=%v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "md0: 0.1\n# External\n" {
		t.Fatalf("stale save overwrote source: %q", got)
	}
}

func TestTerminalEditorCompletionCoversDirectivesAndSymbols(t *testing.T) {
	editor, _ := testTerminalEditor(t, "md0: 0.1\n@in\n@input total number = 2\nValue: {{ to\n")
	editor.cursorY, editor.cursorX = 1, len(editor.lines[1])
	_, _ = editor.handle(editorEvent{key: editorKeyComplete})
	if !editor.completionOn || len(editor.completion.items) == 0 || editor.completion.items[0].label != "@input" {
		t.Fatalf("directive completions=%+v", editor.completion.items)
	}
	_, _ = editor.handle(editorEvent{key: editorKeyEnter})
	if editor.lines[1] == nil || string(editor.lines[1]) != "@input " {
		t.Fatalf("accepted directive=%q", editor.lines[1])
	}

	editor.cursorY, editor.cursorX = 3, len(editor.lines[3])
	items, start := editor.completions(false)
	if start != len([]rune("Value: {{ ")) || !completionLabelsContain(items, "total") {
		t.Fatalf("expression completions start=%d items=%+v", start, items)
	}
}

func TestTerminalEditorBareAtOpensDirectiveCompletions(t *testing.T) {
	editor, _ := testTerminalEditor(t, "@")
	editor.cursorX = 1
	items, start := editor.completions(false)
	if start != 0 || len(items) == 0 || items[0].label != "@input" {
		t.Fatalf("bare @ completions start=%d items=%+v", start, items)
	}
}

func TestTerminalEditorCompletionUsesRuneOffsets(t *testing.T) {
	editor, _ := testTerminalEditor(t, "é @in")
	editor.cursorX = len(editor.lines[0])
	items, start := editor.completions(false)
	if len(items) == 0 || items[0].label != "@input" {
		t.Fatalf("completions=%+v", items)
	}
	if start != len([]rune("é ")) {
		t.Fatalf("replacement start=%d", start)
	}
}

func completionLabelsContain(items []editorCompletion, want string) bool {
	for _, item := range items {
		if item.label == want {
			return true
		}
	}
	return false
}

func TestTerminalEditorQuitConfirmsUnsavedChanges(t *testing.T) {
	editor, _ := testTerminalEditor(t, "md0: 0.1\n")
	_, _ = editor.handle(editorEvent{key: editorKeyRune, r: '#'})
	quit, err := editor.handle(editorEvent{key: editorKeyQuit})
	if quit || err != nil || !editor.confirmQuit {
		t.Fatalf("first quit quit=%v err=%v confirm=%v", quit, err, editor.confirmQuit)
	}
	quit, err = editor.handle(editorEvent{key: editorKeyQuit})
	if !quit || err != nil {
		t.Fatalf("second quit quit=%v err=%v", quit, err)
	}
}

func TestReadEditorEvent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  editorKey
	}{
		{name: "up", input: "\x1b[A", want: editorKeyUp},
		{name: "down", input: "\x1bOB", want: editorKeyDown},
		{name: "delete", input: "\x1b[3~", want: editorKeyDelete},
		{name: "save", input: "\x13", want: editorKeySave},
		{name: "complete", input: "\x00", want: editorKeyComplete},
		{name: "quit", input: "\x11", want: editorKeyQuit},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := readEditorEvent(bufio.NewReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}
			if event.key != test.want {
				t.Fatalf("event=%v, want %v", event.key, test.want)
			}
		})
	}
}

func TestTerminalEditorHighlightsMd0Syntax(t *testing.T) {
	state := editorSyntaxState{}
	styled := highlightTerminalLine("@input amount number = 2", &state, defaultTerminalPalette())
	if !styledHasStyle(styled, "@input", ansiBold+ansiCoral) {
		t.Fatalf("directive was not highlighted: %+v", styled)
	}
	if !styledHasStyle(styled, "number", ansiGreen) || !styledHasStyle(styled, "2", ansiSand) {
		t.Fatalf("typed tokens were not highlighted: %+v", styled)
	}
	styled = highlightTerminalLine("@calc total = 2+3", &state, defaultTerminalPalette())
	if !styledHasStyle(styled, "+", ansiDim) || !styledHasStyle(styled, "2", ansiSand) || !styledHasStyle(styled, "3", ansiSand) {
		t.Fatalf("expression tokens were not highlighted: %+v", styled)
	}
}

func styledHasStyle(styled []editorStyledRune, value, style string) bool {
	for index := 0; index+len([]rune(value)) <= len(styled); index++ {
		if stringStyled(styled[index:index+len([]rune(value))]) == value && styled[index].style == style {
			return true
		}
	}
	return false
}
