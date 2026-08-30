package main

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type editorKey int

const (
	editorKeyRune editorKey = iota
	editorKeyUp
	editorKeyDown
	editorKeyLeft
	editorKeyRight
	editorKeyHome
	editorKeyEnd
	editorKeyPageUp
	editorKeyPageDown
	editorKeyBackspace
	editorKeyDelete
	editorKeyEnter
	editorKeyTab
	editorKeyBacktab
	editorKeyEscape
	editorKeySave
	editorKeyComplete
	editorKeyQuit
)

type editorEvent struct {
	key editorKey
	r   rune
}

type editorCompletion struct {
	label      string
	insert     string
	detail     string
	cursorBack int
}

type editorCompletionMenu struct {
	items        []editorCompletion
	selected     int
	replaceStart int
	force        bool
}

type terminalEditor struct {
	path         string
	lines        [][]rune
	lineEnding   string
	cursorX      int
	cursorY      int
	top          int
	left         int
	dirty        bool
	confirmQuit  bool
	status       string
	statusError  bool
	baseRevision [sha256.Size]byte
	completion   editorCompletionMenu
	completionOn bool
	terminal     terminalUI
	lastWidth    int
	lastHeight   int
	lastCodeRows int
}

type editorSyntaxState struct {
	fence string
	block string
}

type editorStyledRune struct {
	r     rune
	style string
}

var (
	editorDirectivePattern = regexp.MustCompile(`@[A-Za-z_][A-Za-z0-9_]*$|@$`)
	editorInputTypePattern = regexp.MustCompile(`@input\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z]*)$`)
	editorDataTypePattern  = regexp.MustCompile(`@data\s+[A-Za-z_][A-Za-z0-9_]*\s+([A-Za-z]*)$`)
	editorWordPattern      = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*$`)
)

var terminalDirectiveCompletions = []editorCompletion{
	{label: "@input", insert: "@input ", detail: "@input name type = default"},
	{label: "@calc", insert: "@calc ", detail: "@calc name = expression"},
	{label: "@show", insert: "@show ", detail: "@show expression"},
	{label: "@when", insert: "@when ", detail: "@when condition … @end"},
	{label: "@assert", insert: "@assert ", detail: "@assert condition … @end"},
	{label: "@data", insert: "@data ", detail: "@data name json|csv"},
	{label: "@table", insert: "@table ", detail: "table block … @end"},
	{label: "@chart", insert: "@chart ", detail: "bar chart block … @end"},
	{label: "@end", insert: "@end", detail: "close current block"},
}

var terminalInputTypeCompletions = completionsFromWords(
	[]string{"number", "integer", "percent", "currency", "boolean", "bool", "string", "text", "duration"},
	"input type",
)

var terminalDataTypeCompletions = completionsFromWords([]string{"json", "csv"}, "attachment format")

var terminalExpressionBuiltins = []string{
	"abs", "avg", "ceil", "column", "columns", "floor", "get", "len", "max", "min", "round", "rows", "sqrt", "sum",
}

var terminalPlotBuiltins = []string{
	"abs", "acos", "asin", "atan", "ceil", "cos", "exp", "floor", "ln", "log", "log10", "max", "min", "pow", "round", "sin", "sqrt", "tan",
}

var terminalTableCompletions = []editorCompletion{
	{label: "columns", insert: "columns = ", detail: "list of column headings"},
	{label: "rows", insert: "rows = ", detail: "list of row expressions"},
	{label: "@end", insert: "@end", detail: "close table"},
}

var terminalChartCompletions = []editorCompletion{
	{label: "type", insert: "type = bar", detail: "chart type in PURE 0.1"},
	{label: "labels", insert: "labels = ", detail: "list of bar labels"},
	{label: "values", insert: "values = ", detail: "list of numeric expressions"},
	{label: "@end", insert: "@end", detail: "close chart"},
}

var terminalPlotCompletions = []editorCompletion{
	{label: "title", insert: "title = ", detail: "visible plot title"},
	{label: "y", insert: "y = sin(x)", detail: "first curve"},
	{label: "y2", insert: "y2 = cos(x)", detail: "second curve"},
	{label: "label", insert: "label = ", detail: "first curve label"},
	{label: "x", insert: "x = [-10, 10]", detail: "horizontal domain"},
	{label: "samples", insert: "samples = 320", detail: "32 through 1024"},
}

func completionsFromWords(words []string, detail string) []editorCompletion {
	items := make([]editorCompletion, 0, len(words))
	for _, word := range words {
		items = append(items, editorCompletion{label: word, insert: word, detail: detail})
	}
	return items
}

func runTerminalEditor(path string) error {
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
	editor := newTerminalEditor(path, string(data), newTerminalUI(os.Stdout))
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
		editor.draw()
		event, err := readEditorEvent(reader)
		if err != nil {
			return err
		}
		quit, err := editor.handle(event)
		if err != nil {
			editor.status = err.Error()
			editor.statusError = true
		}
		if quit {
			return nil
		}
	}
}

func newTerminalEditor(path, source string, terminal terminalUI) *terminalEditor {
	lineEnding := "\n"
	normalized := source
	if strings.Contains(source, "\r\n") {
		lineEnding = "\r\n"
		normalized = strings.ReplaceAll(source, "\r\n", "\n")
	}
	parts := strings.Split(normalized, "\n")
	lines := make([][]rune, len(parts))
	for index, line := range parts {
		lines[index] = []rune(line)
	}
	if len(lines) == 0 {
		lines = [][]rune{{}}
	}
	return &terminalEditor{
		path:         path,
		lines:        lines,
		lineEnding:   lineEnding,
		status:       "ready",
		baseRevision: sha256.Sum256([]byte(source)),
		terminal:     terminal,
	}
}

func (e *terminalEditor) source() string {
	lines := make([]string, len(e.lines))
	for index, line := range e.lines {
		lines[index] = string(line)
	}
	return strings.Join(lines, e.lineEnding)
}

func (e *terminalEditor) handle(event editorEvent) (bool, error) {
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
			e.acceptCompletion()
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
		e.afterEdit()
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
		e.afterEdit()
	case editorKeyDelete:
		e.deleteForward()
		e.afterEdit()
	case editorKeyEnter:
		e.insertNewline()
		e.afterEdit()
	case editorKeyTab:
		e.insertText("  ")
		e.afterEdit()
	case editorKeyBacktab:
		e.outdent()
		e.afterEdit()
	case editorKeyComplete:
		e.refreshCompletion(true)
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

func (e *terminalEditor) afterEdit() {
	e.dirty = true
	e.status = "modified"
	e.statusError = false
	e.refreshCompletion(false)
}

func (e *terminalEditor) insertRune(value rune) {
	line := e.lines[e.cursorY]
	line = append(line, 0)
	copy(line[e.cursorX+1:], line[e.cursorX:])
	line[e.cursorX] = value
	e.lines[e.cursorY] = line
	e.cursorX++
}

func (e *terminalEditor) insertText(value string) {
	for _, r := range value {
		e.insertRune(r)
	}
}

func (e *terminalEditor) insertNewline() {
	line := e.lines[e.cursorY]
	before := append([]rune(nil), line[:e.cursorX]...)
	after := append([]rune(nil), line[e.cursorX:]...)
	indent := leadingWhitespace(before)
	e.lines[e.cursorY] = before
	newLine := append(append([]rune(nil), indent...), after...)
	e.lines = append(e.lines, nil)
	copy(e.lines[e.cursorY+2:], e.lines[e.cursorY+1:])
	e.lines[e.cursorY+1] = newLine
	e.cursorY++
	e.cursorX = len(indent)
}

func leadingWhitespace(line []rune) []rune {
	end := 0
	for end < len(line) && (line[end] == ' ' || line[end] == '\t') {
		end++
	}
	return append([]rune(nil), line[:end]...)
}

func (e *terminalEditor) backspace() {
	if e.cursorX > 0 {
		line := e.lines[e.cursorY]
		copy(line[e.cursorX-1:], line[e.cursorX:])
		e.lines[e.cursorY] = line[:len(line)-1]
		e.cursorX--
		return
	}
	if e.cursorY == 0 {
		return
	}
	previousLength := len(e.lines[e.cursorY-1])
	e.lines[e.cursorY-1] = append(e.lines[e.cursorY-1], e.lines[e.cursorY]...)
	copy(e.lines[e.cursorY:], e.lines[e.cursorY+1:])
	e.lines = e.lines[:len(e.lines)-1]
	e.cursorY--
	e.cursorX = previousLength
}

func (e *terminalEditor) deleteForward() {
	line := e.lines[e.cursorY]
	if e.cursorX < len(line) {
		copy(line[e.cursorX:], line[e.cursorX+1:])
		e.lines[e.cursorY] = line[:len(line)-1]
		return
	}
	if e.cursorY+1 >= len(e.lines) {
		return
	}
	e.lines[e.cursorY] = append(line, e.lines[e.cursorY+1]...)
	copy(e.lines[e.cursorY+1:], e.lines[e.cursorY+2:])
	e.lines = e.lines[:len(e.lines)-1]
}

func (e *terminalEditor) outdent() {
	line := e.lines[e.cursorY]
	removed := 0
	for removed < 2 && removed < len(line) && line[removed] == ' ' {
		removed++
	}
	if removed == 0 {
		return
	}
	e.lines[e.cursorY] = append([]rune(nil), line[removed:]...)
	e.cursorX = max(0, e.cursorX-removed)
}

func (e *terminalEditor) moveLeft() {
	if e.cursorX > 0 {
		e.cursorX--
		return
	}
	if e.cursorY > 0 {
		e.cursorY--
		e.cursorX = len(e.lines[e.cursorY])
	}
}

func (e *terminalEditor) moveRight() {
	if e.cursorX < len(e.lines[e.cursorY]) {
		e.cursorX++
		return
	}
	if e.cursorY+1 < len(e.lines) {
		e.cursorY++
		e.cursorX = 0
	}
}

func (e *terminalEditor) moveVertical(delta int) {
	e.cursorY = min(max(0, e.cursorY+delta), len(e.lines)-1)
	e.cursorX = min(e.cursorX, len(e.lines[e.cursorY]))
}

func (e *terminalEditor) ensureCursor() {
	e.cursorY = min(max(0, e.cursorY), len(e.lines)-1)
	e.cursorX = min(max(0, e.cursorX), len(e.lines[e.cursorY]))
}

func (e *terminalEditor) save() error {
	current, err := os.ReadFile(e.path)
	if err != nil {
		return err
	}
	if sha256.Sum256(current) != e.baseRevision {
		return errors.New("source changed on disk · quit and reopen before saving")
	}
	info, err := os.Stat(e.path)
	if err != nil {
		return err
	}
	data := []byte(e.source())
	if len(data) > 2*1024*1024 {
		return errors.New("document exceeds 2 MiB limit")
	}
	if err := os.WriteFile(e.path, data, info.Mode().Perm()); err != nil {
		return err
	}
	e.baseRevision = sha256.Sum256(data)
	e.dirty = false
	e.status = "saved"
	e.statusError = false
	return nil
}

func (e *terminalEditor) currentPrefix() string {
	return string(e.lines[e.cursorY][:e.cursorX])
}

func (e *terminalEditor) refreshCompletion(force bool) {
	items, start := e.completions(force)
	if len(items) == 0 {
		e.hideCompletion()
		return
	}
	e.completion = editorCompletionMenu{items: items, replaceStart: start, force: force}
	e.completionOn = true
}

func (e *terminalEditor) completions(force bool) ([]editorCompletion, int) {
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

	wordStart := e.cursorX
	query := ""
	if match := editorWordPattern.FindStringIndex(prefix); match != nil {
		wordStart = utf8.RuneCountInString(prefix[:match[0]])
		query = strings.ToLower(prefix[match[0]:])
	}
	switch e.blockAtCursor() {
	case "plot":
		items := append([]editorCompletion(nil), terminalPlotCompletions...)
		items = append(items, functionCompletions(terminalPlotBuiltins, "plot function")...)
		return filterCompletions(items, query), wordStart
	case "table":
		return filterCompletions(terminalTableCompletions, query), wordStart
	case "chart":
		return filterCompletions(terminalChartCompletions, query), wordStart
	}
	if expressionContext(prefix) {
		items := append(e.symbolCompletions(), functionCompletions(terminalExpressionBuiltins, "md0 builtin")...)
		return filterCompletions(items, query), wordStart
	}
	if force {
		items := append([]editorCompletion{{label: "md0: 0.1", insert: "md0: 0.1", detail: "language declaration"}}, terminalDirectiveCompletions...)
		items = append(items, e.symbolCompletions()...)
		items = append(items, functionCompletions(terminalExpressionBuiltins, "md0 builtin")...)
		return filterCompletions(items, query), wordStart
	}
	return nil, e.cursorX
}

func expressionContext(prefix string) bool {
	if strings.LastIndex(prefix, "{{") > strings.LastIndex(prefix, "}}") {
		return true
	}
	for _, directive := range []string{"@calc", "@show", "@when", "@assert"} {
		if strings.Contains(prefix, directive) {
			return true
		}
	}
	return strings.Contains(prefix, "@input") && strings.Contains(prefix, "=")
}

func (e *terminalEditor) blockAtCursor() string {
	block := ""
	fence := ""
	for lineIndex := 0; lineIndex <= e.cursorY; lineIndex++ {
		line := strings.TrimSpace(string(e.lines[lineIndex]))
		if strings.HasPrefix(line, "```") {
			if fence != "" {
				fence = ""
			} else {
				info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if info == "plot" || info == "md0-plot" {
					fence = "plot"
				} else {
					fence = "code"
				}
			}
			continue
		}
		if fence != "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "@table "):
			block = "table"
		case strings.HasPrefix(line, "@chart "):
			block = "chart"
		case line == "@end":
			block = ""
		}
	}
	if fence != "" {
		return fence
	}
	return block
}

func (e *terminalEditor) symbolCompletions() []editorCompletion {
	seen := map[string]bool{}
	for _, line := range e.lines {
		fields := strings.Fields(string(line))
		for index, field := range fields {
			if (field == "@input" || field == "@calc" || field == "@data") && index+1 < len(fields) {
				name := strings.Trim(fields[index+1], " =")
				if validEditorSymbol(name) {
					seen[name] = true
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return completionsFromWords(names, "document value")
}

func validEditorSymbol(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 && r != '_' && !unicode.IsLetter(r) {
			return false
		}
		if index > 0 && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func functionCompletions(names []string, detail string) []editorCompletion {
	items := make([]editorCompletion, 0, len(names))
	for _, name := range names {
		items = append(items, editorCompletion{label: name, insert: name + "()", detail: detail, cursorBack: 1})
	}
	return items
}

func filterCompletions(items []editorCompletion, query string) []editorCompletion {
	filtered := make([]editorCompletion, 0, len(items))
	for _, item := range items {
		if query == "" || strings.HasPrefix(strings.ToLower(item.label), query) {
			filtered = append(filtered, item)
		}
		if len(filtered) == 12 {
			break
		}
	}
	return filtered
}

func (e *terminalEditor) moveCompletion(delta int) {
	if len(e.completion.items) == 0 {
		return
	}
	e.completion.selected = (e.completion.selected + delta + len(e.completion.items)) % len(e.completion.items)
}

func (e *terminalEditor) hideCompletion() {
	e.completionOn = false
	e.completion = editorCompletionMenu{}
}

func (e *terminalEditor) acceptCompletion() {
	if !e.completionOn || len(e.completion.items) == 0 {
		return
	}
	item := e.completion.items[e.completion.selected]
	line := e.lines[e.cursorY]
	start := min(max(0, e.completion.replaceStart), e.cursorX)
	replacement := []rune(item.insert)
	updated := make([]rune, 0, len(line)-(e.cursorX-start)+len(replacement))
	updated = append(updated, line[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, line[e.cursorX:]...)
	e.lines[e.cursorY] = updated
	e.cursorX = start + len(replacement) - item.cursorBack
	e.hideCompletion()
	e.afterEdit()
}

func readEditorEvent(reader *bufio.Reader) (editorEvent, error) {
	r, _, err := reader.ReadRune()
	if err != nil {
		return editorEvent{}, err
	}
	switch r {
	case 0:
		return editorEvent{key: editorKeyComplete}, nil
	case 3, 17:
		return editorEvent{key: editorKeyQuit}, nil
	case 19:
		return editorEvent{key: editorKeySave}, nil
	case '\r', '\n':
		return editorEvent{key: editorKeyEnter}, nil
	case '\t':
		return editorEvent{key: editorKeyTab}, nil
	case 8, 127:
		return editorEvent{key: editorKeyBackspace}, nil
	case 0x1b:
		return readEditorEscape(reader)
	default:
		if unicode.IsControl(r) {
			return editorEvent{key: editorKeyEscape}, nil
		}
		return editorEvent{key: editorKeyRune, r: r}, nil
	}
}

func readEditorEscape(reader *bufio.Reader) (editorEvent, error) {
	if reader.Buffered() == 0 {
		return editorEvent{key: editorKeyEscape}, nil
	}
	prefix, err := reader.ReadByte()
	if err != nil {
		return editorEvent{}, err
	}
	if prefix != '[' && prefix != 'O' {
		return editorEvent{key: editorKeyEscape}, nil
	}
	sequence := make([]byte, 0, 4)
	for len(sequence) < 8 {
		value, err := reader.ReadByte()
		if err != nil {
			return editorEvent{}, err
		}
		sequence = append(sequence, value)
		if (value >= 'A' && value <= 'Z') || value == '~' {
			break
		}
	}
	switch string(sequence) {
	case "A":
		return editorEvent{key: editorKeyUp}, nil
	case "B":
		return editorEvent{key: editorKeyDown}, nil
	case "C":
		return editorEvent{key: editorKeyRight}, nil
	case "D":
		return editorEvent{key: editorKeyLeft}, nil
	case "H", "1~", "7~":
		return editorEvent{key: editorKeyHome}, nil
	case "F", "4~", "8~":
		return editorEvent{key: editorKeyEnd}, nil
	case "3~":
		return editorEvent{key: editorKeyDelete}, nil
	case "5~":
		return editorEvent{key: editorKeyPageUp}, nil
	case "6~":
		return editorEvent{key: editorKeyPageDown}, nil
	case "Z":
		return editorEvent{key: editorKeyBacktab}, nil
	default:
		return editorEvent{key: editorKeyEscape}, nil
	}
}

func (e *terminalEditor) draw() {
	width, height := terminalSize(os.Stdout)
	width = max(width, 40)
	height = max(height, 10)
	completionRows := 0
	completionLimit := min(6, max(1, height-7))
	if e.completionOn {
		completionRows = min(completionLimit, len(e.completion.items)) + 1
	}
	codeRows := max(3, height-3-completionRows)
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
			content = e.terminal.paint(numberStyle, gutter) + renderStyledRunes(styled[lineIndex], e.left, codeWidth, e.terminal.color)
		}
		e.writeScreenRow(&out, 2+row, content)
	}
	if e.completionOn {
		row := 2 + codeRows
		e.writeScreenRow(&out, row, e.terminal.paint(ansiBold+palette.success, terminalClip(" completions · ↑↓ choose · Enter/Tab insert · Esc close", width)))
		visible := e.visibleCompletions(completionLimit)
		for index, entry := range visible {
			item := e.completion.items[entry]
			marker := "   "
			style := ansiDim
			if entry == e.completion.selected {
				marker = " › "
				style = ansiBold + palette.accent
			}
			labelWidth := min(20, max(8, width/4))
			plain := marker + fmt.Sprintf("%-*s", labelWidth, terminalClip(item.label, labelWidth)) + " " + item.detail
			e.writeScreenRow(&out, row+1+index, e.terminal.paint(style, terminalClip(plain, width)))
		}
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
	e.writeScreenRow(&out, height, e.terminal.paint(ansiDim, terminalClip(help, width)))
	cursorRow := 2 + (e.cursorY - e.top)
	cursorColumn := gutterWidth + 1 + (e.cursorX - e.left)
	cursorColumn = min(max(gutterWidth+1, cursorColumn), width)
	fmt.Fprintf(&out, "\x1b[%d;%dH\x1b[?25h", cursorRow, cursorColumn)
	_, _ = fmt.Fprint(e.terminal.out, out.String())
}

func fallbackTerminalSize() (int, int) {
	width, height := 100, 30
	if parsed, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && parsed > 0 {
		width = parsed
	}
	if parsed, err := strconv.Atoi(os.Getenv("LINES")); err == nil && parsed > 0 {
		height = parsed
	}
	return width, height
}

func (e *terminalEditor) writeScreenRow(out *strings.Builder, row int, content string) {
	fmt.Fprintf(out, "\x1b[%d;1H\x1b[2K%s", row, content)
}

func (e *terminalEditor) ensureViewport(width, codeRows int) {
	if e.cursorY < e.top {
		e.top = e.cursorY
	}
	if e.cursorY >= e.top+codeRows {
		e.top = e.cursorY - codeRows + 1
	}
	digits := len(fmt.Sprint(len(e.lines)))
	codeWidth := max(1, width-(digits+3))
	if e.cursorX < e.left {
		e.left = e.cursorX
	}
	if e.cursorX >= e.left+codeWidth {
		e.left = e.cursorX - codeWidth + 1
	}
}

func (e *terminalEditor) visibleCompletions(limit int) []int {
	if len(e.completion.items) <= limit {
		items := make([]int, len(e.completion.items))
		for index := range items {
			items[index] = index
		}
		return items
	}
	start := e.completion.selected - limit/2
	start = min(max(0, start), len(e.completion.items)-limit)
	items := make([]int, limit)
	for index := range items {
		items[index] = start + index
	}
	return items
}

func terminalClip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:width])
}

func fitTerminalSides(left, right string, width int) string {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes)+len(rightRunes) >= width {
		return terminalClip(left, width)
	}
	return left + strings.Repeat(" ", width-len(leftRunes)-len(rightRunes)) + right
}

func renderStyledRunes(line []editorStyledRune, left, width int, color bool) string {
	if left >= len(line) || width <= 0 {
		return ""
	}
	end := min(len(line), left+width)
	var out strings.Builder
	current := ""
	for _, item := range line[left:end] {
		style := item.style
		if !color {
			style = ""
		}
		if style != current {
			if current != "" {
				out.WriteString(ansiReset)
			}
			if style != "" {
				out.WriteString(style)
			}
			current = style
		}
		out.WriteRune(item.r)
	}
	if current != "" {
		out.WriteString(ansiReset)
	}
	return out.String()
}

func (e *terminalEditor) highlightedLines() [][]editorStyledRune {
	state := editorSyntaxState{}
	output := make([][]editorStyledRune, len(e.lines))
	for index, source := range e.lines {
		line := strings.ReplaceAll(safeTerminalText(string(source)), "\t", "  ")
		output[index] = highlightTerminalLine(line, &state, e.terminal.colors())
	}
	return output
}

func highlightTerminalLine(line string, state *editorSyntaxState, palette terminalPalette) []editorStyledRune {
	runes := []rune(line)
	styled := make([]editorStyledRune, len(runes))
	for index, r := range runes {
		styled[index] = editorStyledRune{r: r}
	}
	trimmed := strings.TrimSpace(line)
	leading := len(runes) - len([]rune(strings.TrimLeftFunc(line, unicode.IsSpace)))
	if strings.HasPrefix(trimmed, "```") {
		setEditorStyle(styled, 0, len(styled), ansiBold+palette.success)
		if state.fence != "" {
			state.fence = ""
		} else {
			info := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if info == "plot" || info == "md0-plot" {
				state.fence = "plot"
			} else {
				state.fence = "code"
			}
		}
		return styled
	}
	if state.fence == "code" {
		setEditorStyle(styled, 0, len(styled), palette.success)
		return styled
	}
	if state.fence == "plot" {
		styleEditorTokens(styled, 0, len(styled), palette, true)
		if equals := indexRune(runes, '='); equals >= 0 {
			setEditorStyle(styled, leading, equals, ansiBold+palette.secondary)
		}
		return styled
	}
	if strings.HasPrefix(trimmed, "md0:") {
		setEditorStyle(styled, leading, len(styled), ansiBold+palette.secondary)
		return styled
	}
	if strings.HasPrefix(trimmed, "#") {
		markerEnd := leading
		for markerEnd < len(runes) && runes[markerEnd] == '#' {
			markerEnd++
		}
		setEditorStyle(styled, leading, markerEnd, palette.secondary)
		setEditorStyle(styled, markerEnd, len(styled), ansiBold)
	}
	styleEditorTokens(styled, 0, len(styled), palette, false)
	for start := 0; start < len(runes); start++ {
		if runes[start] != '@' {
			continue
		}
		end := start + 1
		for end < len(runes) && (runes[end] == '_' || unicode.IsLetter(runes[end])) {
			end++
		}
		name := string(runes[start:end])
		if isEditorDirective(name) {
			setEditorStyle(styled, start, end, ansiBold+palette.accent)
		}
	}
	for start := 0; start+1 < len(runes); start++ {
		if runes[start] == '{' && runes[start+1] == '{' {
			setEditorStyle(styled, start, start+2, ansiBold+palette.accent)
		}
		if runes[start] == '}' && runes[start+1] == '}' {
			setEditorStyle(styled, start, start+2, ansiBold+palette.accent)
		}
	}
	switch {
	case strings.HasPrefix(trimmed, "@table "):
		state.block = "table"
	case strings.HasPrefix(trimmed, "@chart "):
		state.block = "chart"
	case trimmed == "@end":
		state.block = ""
	}
	if (state.block == "table" || state.block == "chart") && !strings.HasPrefix(trimmed, "@") {
		if equals := indexRune(runes, '='); equals >= 0 {
			setEditorStyle(styled, leading, equals, ansiBold+palette.secondary)
		}
	}
	return styled
}

func styleEditorTokens(styled []editorStyledRune, start, end int, palette terminalPalette, plot bool) {
	for index := start; index < end; {
		r := styled[index].r
		switch {
		case r == '"' || r == '\'':
			quote := r
			next := index + 1
			for next < end {
				if styled[next].r == quote && styled[next-1].r != '\\' {
					next++
					break
				}
				next++
			}
			setEditorStyle(styled, index, next, palette.success)
			index = next
		case unicode.IsDigit(r):
			next := editorNumberEnd(styled, index, end)
			setEditorStyle(styled, index, next, palette.secondary)
			index = next
		case unicode.IsLetter(r) || r == '_':
			next := index + 1
			for next < end && (unicode.IsLetter(styled[next].r) || unicode.IsDigit(styled[next].r) || styled[next].r == '_') {
				next++
			}
			word := stringStyled(styled[index:next])
			if isEditorBuiltin(word, plot) || isEditorType(word) {
				setEditorStyle(styled, index, next, palette.success)
			} else if word == "true" || word == "false" || word == "null" {
				setEditorStyle(styled, index, next, palette.accent)
			}
			index = next
		case strings.ContainsRune("+-*/%<>=!?:&|", r):
			styled[index].style = ansiDim
			index++
		default:
			index++
		}
	}
}

func editorNumberEnd(styled []editorStyledRune, start, end int) int {
	next := start
	for next < end && unicode.IsDigit(styled[next].r) {
		next++
	}
	if next < end && styled[next].r == '.' {
		next++
		for next < end && unicode.IsDigit(styled[next].r) {
			next++
		}
	}
	if next < end && (styled[next].r == 'e' || styled[next].r == 'E') {
		next++
		if next < end && (styled[next].r == '+' || styled[next].r == '-') {
			next++
		}
		for next < end && unicode.IsDigit(styled[next].r) {
			next++
		}
	}
	for next < end && unicode.IsLetter(styled[next].r) {
		next++
	}
	return next
}

func setEditorStyle(styled []editorStyledRune, start, end int, style string) {
	start = max(0, start)
	end = min(len(styled), end)
	for index := start; index < end; index++ {
		styled[index].style = style
	}
}

func stringStyled(items []editorStyledRune) string {
	var value strings.Builder
	for _, item := range items {
		value.WriteRune(item.r)
	}
	return value.String()
}

func indexRune(value []rune, needle rune) int {
	for index, r := range value {
		if r == needle {
			return index
		}
	}
	return -1
}

func isEditorDirective(value string) bool {
	for _, item := range terminalDirectiveCompletions {
		if item.label == value {
			return true
		}
	}
	return false
}

func isEditorBuiltin(value string, plot bool) bool {
	items := terminalExpressionBuiltins
	if plot {
		items = terminalPlotBuiltins
	}
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func isEditorType(value string) bool {
	for _, item := range terminalInputTypeCompletions {
		if item.label == value {
			return true
		}
	}
	for _, item := range terminalDataTypeCompletions {
		if item.label == value {
			return true
		}
	}
	return false
}
