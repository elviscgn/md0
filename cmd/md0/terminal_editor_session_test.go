package main

import (
	"bufio"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadEditorSessionControlKeys(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want editorSessionSpecial
	}{
		{name: "find", in: "\x06", want: editorSessionFind},
		{name: "redo", in: "\x19", want: editorSessionRedo},
		{name: "undo", in: "\x1a", want: editorSessionUndo},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := readEditorSessionEvent(bufio.NewReader(strings.NewReader(test.in)))
			if err != nil {
				t.Fatal(err)
			}
			if event.special != test.want {
				t.Fatalf("special=%v, want %v", event.special, test.want)
			}
		})
	}
}

func TestReadLauncherHelpKey(t *testing.T) {
	key, err := readLauncherKey(bufio.NewReader(strings.NewReader("?")))
	if err != nil {
		t.Fatal(err)
	}
	if key != launcherKeyHelp {
		t.Fatalf("key=%v, want help", key)
	}
}

func TestEditorSessionUndoRedo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	session := newTerminalEditorSession(path, "abc", terminalUI{})
	defer session.close()

	if _, err := session.handle(editorSessionEvent{event: editorEvent{key: editorKeyRune, r: 'x'}}); err != nil {
		t.Fatal(err)
	}
	if got := session.source(); got != "xabc" {
		t.Fatalf("edited source=%q", got)
	}
	session.undoEdit()
	if got := session.source(); got != "abc" {
		t.Fatalf("undo source=%q", got)
	}
	session.redoEdit()
	if got := session.source(); got != "xabc" {
		t.Fatalf("redo source=%q", got)
	}
}

func TestEditorSessionCoalescesTypingUndo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	session := newTerminalEditorSession(path, "", terminalUI{})
	defer session.close()

	for _, r := range "hello" {
		if _, err := session.handle(editorSessionEvent{event: editorEvent{key: editorKeyRune, r: r}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(session.undo.items) != 1 {
		t.Fatalf("undo states=%d, want 1 typing group", len(session.undo.items))
	}
	session.undoEdit()
	if got := session.source(); got != "" {
		t.Fatalf("undo source=%q", got)
	}
}

func TestEditorFindMatchesCaseInsensitive(t *testing.T) {
	lines := [][]rune{[]rune("Alpha beta alpha"), []rune("ALPHA")}
	matches := editorFindMatches(lines, []rune("alpha"))
	if len(matches) != 3 {
		t.Fatalf("matches=%d, want 3", len(matches))
	}
	if matches[1].line != 0 || matches[1].column != 11 {
		t.Fatalf("second match=%+v", matches[1])
	}
}

func TestEditorAutosaveDebouncesAndWritesLatest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	autosaver := newEditorAutosaver(path, sha256.Sum256([]byte("base")), 15*time.Millisecond)
	defer autosaver.Stop()
	autosaver.Schedule("first")
	autosaver.Schedule("latest")

	select {
	case result := <-autosaver.results:
		if result.err != nil {
			t.Fatal(result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for autosave")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "latest" {
		t.Fatalf("saved=%q, want latest", got)
	}
}

func TestEditorAutosaveCanSaveEmptyDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	autosaver := newEditorAutosaver(path, sha256.Sum256([]byte("base")), 10*time.Millisecond)
	defer autosaver.Stop()
	autosaver.Schedule("")

	select {
	case result := <-autosaver.results:
		if result.err != nil {
			t.Fatal(result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for empty autosave")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("saved %q, want empty document", string(data))
	}
}

func TestEditorAutosaveRejectsExternalChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	autosaver := newEditorAutosaver(path, sha256.Sum256([]byte("base")), time.Hour)
	defer autosaver.Stop()
	if err := os.WriteFile(path, []byte("external"), 0600); err != nil {
		t.Fatal(err)
	}
	result := autosaver.SaveNow("mine")
	if result.err == nil {
		t.Fatal("expected stale-save rejection")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "external" {
		t.Fatalf("external source overwritten: %q", got)
	}
}

func TestEditorEscapeForcesPendingSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	session := newTerminalEditorSession(path, "base", terminalUI{})
	defer session.close()
	if _, err := session.handle(editorSessionEvent{event: editorEvent{key: editorKeyRune, r: 'x'}}); err != nil {
		t.Fatal(err)
	}
	back, err := session.handle(editorSessionEvent{event: editorEvent{key: editorKeyEscape}})
	if err != nil {
		t.Fatal(err)
	}
	if !back {
		t.Fatal("Esc should save pending edits and leave editor")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "xbase" {
		t.Fatalf("saved=%q", got)
	}
}
