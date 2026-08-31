package main

import (
	"fmt"
	"os"
	"strings"
)

type editorShortcut struct {
	key   string
	label string
}

func (e *terminalEditorUX) drawPolishedUX(navigationLabel string) {
	width, height := terminalSize(os.Stdout)
	width = max(width, 40)
	height = max(height, 10)
	codeRows := max(3, height-3)
	e.lastWidth, e.lastHeight, e.lastCodeRows = width, height, codeRows
	e.ensureViewport(width, codeRows)
	styled := e.highlightedLines()
	digits := len(fmt.Sprint(len(e.lines)))
	gutterWidth := digits + 3
	codeWidth := max(1, width-gutterWidth)
	palette := e.terminal.colors()

	var out strings.Builder
	out.WriteString("\x1b[?25l")
	e.writeScreenRow(&out, 1, e.editorHeader(width, palette))

	for row := 0; row < codeRows; row++ {
		lineIndex := e.top + row
		content := ""
		if lineIndex < len(e.lines) {
			numberStyle := ansiDim
			if lineIndex == e.cursorY {
				numberStyle = ansiBold + palette.accent
			}
			gutter := fmt.Sprintf("%*d │ ", digits, lineIndex+1)
			rendered := renderStyledRunes(styled[lineIndex], e.left, codeWidth, e.terminal.color)
			if lineIndex == e.cursorY {
				ghost := e.completionGhost()
				visibleRunes := max(0, len(e.lines[lineIndex])-e.left)
				remaining := codeWidth - visibleRunes
				if ghost != "" && remaining > 0 {
					rendered += e.terminal.paint(ansiDim, terminalClip(ghost, remaining))
				}
			}
			content = e.terminal.paint(numberStyle, gutter) + rendered
		}
		e.writeScreenRow(&out, 2+row, content)
	}

	statusStyle := ansiDim
	if e.statusError {
		statusStyle = ansiBold + palette.error
	} else if e.status == "saved" {
		statusStyle = ansiBold + palette.success
	}
	lineEnding := "LF"
	if e.lineEnding == "\r\n" {
		lineEnding = "CRLF"
	}
	position := fmt.Sprintf("Ln %d  Col %d  %s  UTF-8", e.cursorY+1, e.cursorX+1, lineEnding)
	status := fitTerminalSides(" "+e.status, position+" ", width)
	e.writeScreenRow(&out, height-1, e.terminal.paint(statusStyle, status))

	shortcuts := []editorShortcut{
		{key: "Esc", label: navigationLabel},
		{key: "Ctrl+S", label: "save"},
		{key: "Ctrl+Z", label: "undo"},
		{key: "Ctrl+F", label: "find"},
		{key: "Ctrl+Space", label: "complete"},
		{key: "Tab", label: "indent"},
	}
	if e.completionOn {
		shortcuts = []editorShortcut{
			{key: "↑↓", label: "choose"},
			{key: "Enter/Tab", label: "insert"},
			{key: "Esc", label: "dismiss"},
			{key: "Ctrl+Space", label: "all"},
		}
	}
	e.writeScreenRow(&out, height, e.editorShortcutBar(width, palette, shortcuts))

	cursorRow := 2 + (e.cursorY - e.top)
	cursorColumn := gutterWidth + 1 + (e.cursorX - e.left)
	cursorColumn = min(max(gutterWidth+1, cursorColumn), width)
	if e.completionOn {
		e.drawCompletionPopup(&out, width, codeRows, cursorRow, cursorColumn, palette)
	}
	fmt.Fprintf(&out, "\x1b[%d;%dH\x1b[?25h", cursorRow, cursorColumn)
	_, _ = fmt.Fprint(e.terminal.out, out.String())
}

func (e *terminalEditorUX) editorHeader(width int, palette terminalPalette) string {
	stateText := "✓ saved"
	stateStyle := ansiBold + palette.success
	if e.dirty {
		stateText = "● unsaved"
		stateStyle = ansiBold + palette.accent
	}

	prefixWidth := len([]rune(" md0/PURE  EDIT  "))
	stateWidth := len([]rune(stateText)) + 1
	pathWidth := max(8, width-prefixWidth-stateWidth-2)
	path := compactTerminalText(safeTerminalText(e.path), pathWidth)
	used := prefixWidth + len([]rune(path)) + stateWidth
	padding := strings.Repeat(" ", max(1, width-used))

	return e.terminal.paint(ansiBold+palette.secondary, " md0/PURE") +
		e.terminal.paint(ansiBold+palette.accent, "  EDIT  ") +
		e.terminal.paint("", path) + padding +
		e.terminal.paint(stateStyle, stateText+" ")
}

func (e *terminalEditorUX) editorShortcutBar(width int, palette terminalPalette, shortcuts []editorShortcut) string {
	var out strings.Builder
	out.WriteString(" ")
	used := 1
	for index, shortcut := range shortcuts {
		separator := ""
		if index > 0 {
			separator = "  ·  "
		}
		plainWidth := len([]rune(separator + shortcut.key + " " + shortcut.label))
		if used+plainWidth > width {
			break
		}
		if separator != "" {
			out.WriteString(e.terminal.paint(ansiDim, separator))
		}
		out.WriteString(e.terminal.paint(ansiBold+palette.accent, shortcut.key))
		out.WriteString(e.terminal.paint(ansiDim, " "+shortcut.label))
		used += plainWidth
	}
	return out.String()
}
