package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const editorPopupMaxItems = 5

type terminalEditorUX struct {
	*terminalEditor
}

type editorSnippet struct {
	source       string
	cursorLine   int
	cursorColumn int
}

func runTerminalEditorUX(path string) error {
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
		editor.drawUX()
		event, err := readEditorEvent(reader)
		if err != nil {
			return err
		}
		quit, err := editor.handleUX(event)
		if err != nil {
			editor.status = err.Error()
			editor.statusError = true
		}
		if quit {
			return nil
		}
	}
}

func (e *terminalEditorUX) handleUX(event editorEvent) (bool, error) {
	if event.key != editorKeyQuit {
		e.confirmQuit = false
	}
	if e.completionOn {
		switch event.key {
		case editorKeyUp:
			e.moveCompletion(-1)
			return false, nil
		case editorKeyDown:
			e.moveCompletion(1)
			return false, nil
		case editorKeyEnter, editorKeyTab:
			e.acceptCompletionUX()
			return false, nil
		case editorKeyEscape:
			e.hideCompletion()
			return false, nil
		case editorKeyLeft, editorKeyRight, editorKeyHome, editorKeyEnd, editorKeyPageUp, editorKeyPageDown, editorKeySave, editorKeyQuit:
			e.hideCompletion()
		}
	}

	switch event.key {
	case editorKeyRune:
		e.insertRune(event.r)
		e.afterEditUX()
	case editorKeyUp:
		e.moveVertical(-1)
	case editorKeyDown:
		e.moveVertical(1)
	case editorKeyLeft:
		e.moveLeft()
	case editorKeyRight:
		e.moveRight()
	case editorKeyHome:
		e.cursorX = 0
	case editorKeyEnd:
		e.cursorX = len(e.lines[e.cursorY])
	case editorKeyPageUp:
		e.moveVertical(-max(1, e.lastCodeRows-1))
	case editorKeyPageDown:
		e.moveVertical(max(1, e.lastCodeRows-1))
	case editorKeyBackspace:
		e.backspace()
		e.afterEditUX()
	case editorKeyDelete:
		e.deleteForward()
		e.afterEditUX()
	case editorKeyEnter:
		e.insertNewline()
		e.afterEditUX()
	case editorKeyTab:
		e.insertText("  ")
		e.afterEditUX()
	case editorKeyBacktab:
		e.outdent()
		e.afterEditUX()
	case editorKeyComplete:
		e.refreshCompletionUX(true)
	case editorKeySave:
		return false, e.save()
	case editorKeyEscape:
		e.hideCompletion()
	case editorKeyQuit:
		if e.dirty && !e.confirmQuit {
			e.confirmQuit = true
			e.status = "unsaved changes · press Ctrl+Q again to discard"
			e.statusError = true
			return false, nil
		}
		return true, nil
	}
	e.ensureCursor()
	return false, nil
}

func (e *terminalEditorUX) afterEditUX() {
	e.dirty = true
	e.status = "modified"
	e.statusError = false
	e.refreshCompletionUX(false)
}

func (e *terminalEditorUX) refreshCompletionUX(force bool) {
	items, start := e.completionsUX(force)
	if len(items) == 0 {
		e.hideCompletion()
		return
	}
	selectedLabel := ""
	if e.completionOn && e.completion.selected >= 0 && e.completion.selected < len(e.completion.items) {
		selectedLabel = e.completion.items[e.completion.selected].label
	}
	e.completion = editorCompletionMenu{items: items, replaceStart: start, force: force}
	for index, item := range items {
		if item.label == selectedLabel {
			e.completion.selected = index
			break
		}
	}
	e.completionOn = true
}

func (e *terminalEditorUX) completionsUX(force bool) ([]editorCompletion, int) {
	prefix := e.currentPrefix()
	if match := editorDirectivePattern.FindStringIndex(prefix); match != nil {
		query := strings.ToLower(prefix[match[0]:])
		return filterCompletions(terminalDirectiveCompletions, query), utf8.RuneCountInString(prefix[:match[0]])
	}
	if match := editorInputTypePattern.FindStringSubmatchIndex(prefix); match != nil {
		query := strings.ToLower(prefix[match[2]:match[3]])
		return filterCompletions(terminalInputTypeCompletions, query), utf8.RuneCountInString(prefix[:match[2]])
	}
	if match := editorDataTypePattern.FindStringSubmatchIndex(prefix); match != nil {
		query := strings.ToLower(prefix[match[2]:match[3]])
		return filterCompletions(terminalDataTypeCompletions, query), utf8.RuneCountInString(prefix[:match[2]])
	}
	if items, start, ok := plotFenceCompletions(prefix, force); ok {
		return items, start
	}

	wordStart := e.cursorX
	query := ""
	if match := editorWordPattern.FindStringIndex(prefix); match != nil {
		wordStart = utf8.RuneCountInString(prefix[:match[0]])
		query = strings.ToLower(prefix[match[0]:])
	}

	switch e.blockAtCursor() {
	case "plot":
		if query == "" && !force {
			return nil, e.cursorX
		}
		items := append([]editorCompletion(nil), terminalPlotCompletions...)
		items = append(items, functionCompletions(terminalPlotBuiltins, "plot function")...)
		items = append(items, e.symbolCompletions()...)
		return filterCompletions(items, query), wordStart
	case "table":
		if query == "" && !force {
			return nil, e.cursorX
		}
		return filterCompletions(terminalTableCompletions, query), wordStart
	case "chart":
		if query == "" && !force {
			return nil, e.cursorX
		}
		return filterCompletions(terminalChartCompletions, query), wordStart
	}
	if expressionContext(prefix) {
		if query == "" && !force {
			return nil, e.cursorX
		}
		items := append(e.symbolCompletions(), functionCompletions(terminalExpressionBuiltins, "md0 builtin")...)
		return filterCompletions(items, query), wordStart
	}
	if force {
		items := []editorCompletion{
			{label: "md0: 0.1", insert: "md0: 0.1", detail: "language declaration"},
			{label: "```plot", insert: "```plot", detail: "reactive SVG function plot"},
		}
		items = append(items, terminalDirectiveCompletions...)
		items = append(items, e.symbolCompletions()...)
		items = append(items, functionCompletions(terminalExpressionBuiltins, "md0 builtin")...)
		return filterCompletions(items, query), wordStart
	}
	return nil, e.cursorX
}

func plotFenceCompletions(prefix string, force bool) ([]editorCompletion, int, bool) {
	trimmed := strings.TrimLeftFunc(prefix, unicode.IsSpace)
	if !strings.HasPrefix(trimmed, "```") {
		return nil, 0, false
	}
	query := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
	if query == "" && !force {
		query = ""
	}
	item := editorCompletion{label: "```plot", insert: "```plot", detail: "reactive SVG function plot"}
	if query != "" && !strings.HasPrefix("plot", query) {
		return nil, 0, true
	}
	start := utf8.RuneCountInString(prefix) - utf8.RuneCountInString(trimmed)
	return []editorCompletion{item}, start, true
}

func (e *terminalEditorUX) acceptCompletionUX() {
	if !e.completionOn || len(e.completion.items) == 0 {
		return
	}
	item := e.completion.items[e.completion.selected]
	e.applyCompletionSnippet(item)
	e.hideCompletion()
	e.afterEditUX()
}

func completionSnippetFor(item editorCompletion) editorSnippet {
	switch item.label {
	case "@when":
		return editorSnippet{source: "@when \n\n@end", cursorLine: 0, cursorColumn: len([]rune("@when "))}
	case "@assert":
		return editorSnippet{source: "@assert \n\n@end", cursorLine: 0, cursorColumn: len([]rune("@assert "))}
	case "@table":
		return editorSnippet{source: "@table \ncolumns = []\nrows = []\n@end", cursorLine: 0, cursorColumn: len([]rune("@table "))}
	case "@chart":
		return editorSnippet{source: "@chart \nlabels = []\nvalues = []\n@end", cursorLine: 0, cursorColumn: len([]rune("@chart "))}
	case "```plot":
		return editorSnippet{source: "```plot\nf(x) = sin(x)\nx = [-2*pi, 2*pi]\n```", cursorLine: 1, cursorColumn: len([]rune("f(x) = "))}
	default:
		column := len([]rune(item.insert)) - item.cursorBack
		return editorSnippet{source: item.insert, cursorLine: 0, cursorColumn: max(0, column)}
	}
}

func (e *terminalEditorUX) applyCompletionSnippet(item editorCompletion) {
	line := e.lines[e.cursorY]
	start := min(max(0, e.completion.replaceStart), e.cursorX)
	prefix := append([]rune(nil), line[:start]...)
	suffix := append([]rune(nil), line[e.cursorX:]...)
	snippet := completionSnippetFor(item)
	parts := strings.Split(snippet.source, "\n")
	indent := leadingWhitespace(prefix)
	inserted := make([][]rune, len(parts))
	for index, part := range parts {
		content := []rune(part)
		if index == 0 {
			content = append(append([]rune(nil), prefix...), content...)
		} else if len(indent) > 0 {
			content = append(append([]rune(nil), indent...), content...)
		}
		if index == len(parts)-1 {
			content = append(content, suffix...)
		}
		inserted[index] = content
	}

	updated := make([][]rune, 0, len(e.lines)-1+len(inserted))
	updated = append(updated, e.lines[:e.cursorY]...)
	updated = append(updated, inserted...)
	updated = append(updated, e.lines[e.cursorY+1:]...)
	originalY := e.cursorY
	e.lines = updated
	e.cursorY = originalY + snippet.cursorLine
	if snippet.cursorLine == 0 {
		e.cursorX = len(prefix) + snippet.cursorColumn
	} else {
		e.cursorX = len(indent) + snippet.cursorColumn
	}
	e.ensureCursor()
}

func (e *terminalEditorUX) completionGhost() string {
	if !e.completionOn || len(e.completion.items) == 0 || e.completion.selected >= len(e.completion.items) {
		return ""
	}
	line := e.lines[e.cursorY]
	if e.cursorX != len(line) {
		return ""
	}
	start := min(max(0, e.completion.replaceStart), e.cursorX)
	typed := string(line[start:e.cursorX])
	preview := completionSnippetFor(e.completion.items[e.completion.selected]).source
	if newline := strings.IndexByte(preview, '\n'); newline >= 0 {
		preview = preview[:newline]
	}
	if typed == "" || !strings.HasPrefix(preview, typed) {
		return ""
	}
	return strings.TrimPrefix(preview, typed)
}

func (e *terminalEditorUX) drawUX() {
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
	header := " md0/PURE · " + compactTerminalText(safeTerminalText(e.path), max(8, width-22))
	if e.dirty {
		header += " · unsaved"
	}
	e.writeScreenRow(&out, 1, e.terminal.paint(ansiBold+palette.accent, terminalClip(header, width)))
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
	position := fmt.Sprintf("Ln %d, Col %d", e.cursorY+1, e.cursorX+1)
	status := fitTerminalSides(" "+e.status, position+" ", width)
	e.writeScreenRow(&out, height-1, e.terminal.paint(statusStyle, status))
	help := " Ctrl+S save · Ctrl+Space complete · Tab indent · Ctrl+Q quit"
	if e.completionOn {
		help = " ↑↓ choose · Enter/Tab insert · Esc close · Ctrl+Space all"
	}
	e.writeScreenRow(&out, height, e.terminal.paint(ansiDim, terminalClip(help, width)))

	cursorRow := 2 + (e.cursorY - e.top)
	cursorColumn := gutterWidth + 1 + (e.cursorX - e.left)
	cursorColumn = min(max(gutterWidth+1, cursorColumn), width)
	if e.completionOn {
		e.drawCompletionPopup(&out, width, codeRows, cursorRow, cursorColumn, palette)
	}
	fmt.Fprintf(&out, "\x1b[%d;%dH\x1b[?25h", cursorRow, cursorColumn)
	_, _ = fmt.Fprint(e.terminal.out, out.String())
}

func (e *terminalEditorUX) drawCompletionPopup(out *strings.Builder, width, codeRows, cursorRow, cursorColumn int, palette terminalPalette) {
	visible := e.visibleCompletions(editorPopupMaxItems)
	if len(visible) == 0 || width < 24 {
		return
	}
	innerWidth := completionPopupWidth(e.completion.items, visible, width)
	boxWidth := innerWidth + 2
	boxHeight := len(visible) + 2
	startRow, startColumn := completionPopupOrigin(width, codeRows, cursorRow, cursorColumn, boxWidth, boxHeight)

	title := fmt.Sprintf(" suggestions %d/%d ", e.completion.selected+1, len(e.completion.items))
	title = terminalClip(title, innerWidth)
	topFill := max(0, innerWidth-len([]rune(title)))
	top := "┌" + title + strings.Repeat("─", topFill) + "┐"
	fmt.Fprintf(out, "\x1b[%d;%dH%s", startRow, startColumn, e.terminal.paint(palette.secondary, top))

	for offset, entry := range visible {
		item := e.completion.items[entry]
		marker := "  "
		style := ansiDim
		if entry == e.completion.selected {
			marker = "› "
			style = ansiBold + palette.accent
		}
		plain := marker + item.label
		if item.detail != "" {
			plain += "  " + item.detail
		}
		plain = terminalClip(plain, innerWidth)
		plain += strings.Repeat(" ", max(0, innerWidth-len([]rune(plain))))
		row := startRow + 1 + offset
		fmt.Fprintf(out, "\x1b[%d;%dH%s%s%s",
			row,
			startColumn,
			e.terminal.paint(palette.secondary, "│"),
			e.terminal.paint(style, plain),
			e.terminal.paint(palette.secondary, "│"),
		)
	}
	bottom := "└" + strings.Repeat("─", innerWidth) + "┘"
	fmt.Fprintf(out, "\x1b[%d;%dH%s", startRow+boxHeight-1, startColumn, e.terminal.paint(palette.secondary, bottom))
}

func completionPopupWidth(items []editorCompletion, visible []int, terminalWidth int) int {
	longest := 0
	for _, index := range visible {
		if index < 0 || index >= len(items) {
			continue
		}
		length := len([]rune(items[index].label)) + len([]rune(items[index].detail)) + 4
		longest = max(longest, length)
	}
	return min(max(28, longest), max(20, min(58, terminalWidth-2)))
}

func completionPopupOrigin(width, codeRows, cursorRow, cursorColumn, boxWidth, boxHeight int) (int, int) {
	firstCodeRow := 2
	lastCodeRow := 1 + codeRows
	startRow := cursorRow + 1
	if startRow+boxHeight-1 > lastCodeRow {
		startRow = cursorRow - boxHeight
	}
	startRow = min(max(firstCodeRow, startRow), max(firstCodeRow, lastCodeRow-boxHeight+1))
	startColumn := cursorColumn
	if startColumn+boxWidth-1 > width {
		startColumn = width - boxWidth + 1
	}
	startColumn = max(1, startColumn)
	return startRow, startColumn
}
