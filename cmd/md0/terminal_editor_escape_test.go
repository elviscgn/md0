package main

import "testing"

func TestEditorEscapeQuitsCleanDocument(t *testing.T) {
	editor := testTerminalEditorUX(t, "md0: 0.1\n")
	quit, err := editor.handleUXEscape(editorEvent{key: editorKeyEscape})
	if err != nil {
		t.Fatal(err)
	}
	if !quit {
		t.Fatal("Esc should quit a clean terminal editor")
	}
}

func TestEditorEscapeRequiresConfirmationForUnsavedChanges(t *testing.T) {
	editor := testTerminalEditorUX(t, "md0: 0.1\n")
	editor.dirty = true

	quit, err := editor.handleUXEscape(editorEvent{key: editorKeyEscape})
	if err != nil {
		t.Fatal(err)
	}
	if quit {
		t.Fatal("first Esc should not discard unsaved changes")
	}
	if !editor.confirmQuit || editor.status != "unsaved changes · press Esc again to discard" {
		t.Fatalf("confirm=%v status=%q", editor.confirmQuit, editor.status)
	}

	quit, err = editor.handleUXEscape(editorEvent{key: editorKeyEscape})
	if err != nil {
		t.Fatal(err)
	}
	if !quit {
		t.Fatal("second Esc should discard unsaved changes and quit")
	}
}

func TestEditorEscapeDismissesCompletionBeforeQuitting(t *testing.T) {
	editor := testTerminalEditorUX(t, "@w")
	editor.cursorX = len(editor.lines[0])
	editor.refreshCompletionUX(false)
	if !editor.completionOn {
		t.Fatal("expected completion popup")
	}

	quit, err := editor.handleUXEscape(editorEvent{key: editorKeyEscape})
	if err != nil {
		t.Fatal(err)
	}
	if quit {
		t.Fatal("Esc should dismiss an open completion before quitting")
	}
	if editor.completionOn {
		t.Fatal("completion popup should be closed")
	}
}
