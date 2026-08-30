package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"
)

func runTerminalEditorUXEscape(path string) error {
	if !launcherAvailable() {
		return errors.New("terminal editor requires an interactive terminal")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return errors.New("document must be valid UTF-8")
	}
	if len(data) > 2*1024*1024 {
		return errors.New("document exceeds 2 MiB limit")
	}

	editor := &terminalEditorUX{terminalEditor: newTerminalEditor(path, string(data), newTerminalUI(os.Stdout))}
	restore, err := enableRawTerminal(os.Stdin)
	if err != nil {
		return fmt.Errorf("terminal editor could not enable interactive input: %w", err)
	}
	fmt.Fprint(editor.terminal.out, "\x1b[?1049h\x1b[2J\x1b[H")
	defer func() {
		_ = restore()
		fmt.Fprint(editor.terminal.out, "\x1b[?25h\x1b[?1049l")
	}()

	reader := bufio.NewReader(os.Stdin)
	for {
		editor.drawPolishedUX("exit")
		event, err := readEditorEvent(reader)
		if err != nil {
			return err
		}
		quit, err := editor.handleUXEscape(event)
		if err != nil {
			editor.status = err.Error()
			editor.statusError = true
		}
		if quit {
			return nil
		}
	}
}

func (e *terminalEditorUX) handleUXEscape(event editorEvent) (bool, error) {
	if event.key != editorKeyEscape || e.completionOn {
		return e.handleUX(event)
	}
	if e.dirty && !e.confirmQuit {
		e.confirmQuit = true
		e.status = "unsaved changes · press Esc again to discard"
		e.statusError = true
		return false, nil
	}
	return true, nil
}
