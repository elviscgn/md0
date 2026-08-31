package main

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	editorAutosaveDelay    = 400 * time.Millisecond
	editorHistoryMaxStates = 128
	editorHistoryMaxBytes  = 16 << 20
)

type editorSessionSpecial int

const (
	editorSessionNone editorSessionSpecial = iota
	editorSessionUndo
	editorSessionRedo
	editorSessionFind
)

type editorSessionEvent struct {
	event   editorEvent
	special editorSessionSpecial
}

type editorEditClass int

const (
	editorEditNone editorEditClass = iota
	editorEditInsert
	editorEditBackspace
	editorEditDelete
	editorEditIndent
	editorEditStructure
	editorEditCompletion
)

type editorSnapshot struct {
	source  string
	cursorX int
	cursorY int
	top     int
	left    int
}

type editorSnapshotStack struct {
	items []editorSnapshot
	bytes int
}

type editorSearchMatch struct {
	line   int
	column int
	length int
}

type editorSearchState struct {
	active  bool
	query   []rune
	matches []editorSearchMatch
	index   int
}

type editorAutosaveCommand struct {
	source   string
	force    bool
	response chan editorAutosaveResult
}

type editorAutosaveResult struct {
	sourceHash [sha256.Size]byte
	revision   [sha256.Size]byte
	err        error
}

type editorAutosaver struct {
	path     string
	delay    time.Duration
	commands chan editorAutosaveCommand
	results  chan editorAutosaveResult
	stop     chan struct{}
	stopped  bool
}

type terminalEditorSession struct {
	*terminalEditorUX
	undo      editorSnapshotStack
	redo      editorSnapshotStack
	lastEdit  editorEditClass
	search    editorSearchState
	autosaver *editorAutosaver
}

func newTerminalEditorSession(path, source string, terminal terminalUI) *terminalEditorSession {
	ux := &terminalEditorUX{terminalEditor: newTerminalEditor(path, source, terminal)}
	return &terminalEditorSession{
		terminalEditorUX: ux,
		autosaver:        newEditorAutosaver(path, ux.baseRevision, editorAutosaveDelay),
	}
}

func (s *terminalEditorSession) close() {
	if s.autosaver != nil {
		s.autosaver.Stop()
	}
}

func readEditorSessionEvent(reader *bufio.Reader) (editorSessionEvent, error) {
	r, _, err := reader.ReadRune()
	if err != nil {
		return editorSessionEvent{}, err
	}
	switch r {
	case 6: // Ctrl+F
		return editorSessionEvent{special: editorSessionFind}, nil
	case 25: // Ctrl+Y
		return editorSessionEvent{special: editorSessionRedo}, nil
	case 26: // Ctrl+Z
		return editorSessionEvent{special: editorSessionUndo}, nil
	}
	if err := reader.UnreadRune(); err != nil {
		return editorSessionEvent{}, err
	}
	event, err := readEditorEvent(reader)
	return editorSessionEvent{event: event}, err
}

func (s *terminalEditorSession) draw(navigationLabel string) {
	s.pollAutosave()
	s.drawPolishedUX(navigationLabel)
	if s.search.active {
		s.drawSearchOverlay()
	}
}

func (s *terminalEditorSession) handle(input editorSessionEvent) (bool, error) {
	s.pollAutosave()
	if s.search.active {
		return s.handleSearch(input)
	}

	switch input.special {
	case editorSessionUndo:
		s.undoEdit()
		return false, nil
	case editorSessionRedo:
		s.redoEdit()
		return false, nil
	case editorSessionFind:
		s.openSearch()
		return false, nil
	}

	event := input.event
	if event.key == editorKeySave {
		s.lastEdit = editorEditNone
		return false, s.saveNow()
	}
	if event.key == editorKeyEscape && !s.completionOn {
		s.lastEdit = editorEditNone
		return s.escapeEditor()
	}
	if event.key == editorKeyQuit {
		s.lastEdit = editorEditNone
		return s.escapeEditor()
	}

	class, mutates := s.editClass(event)
	if class != editorEditNone && !mutates {
		s.lastEdit = editorEditNone
		return false, nil
	}
	if mutates {
		s.recordBeforeEdit(class)
	} else if event.key != editorKeyUp && event.key != editorKeyDown && s.completionOn {
		// Completion navigation should not split a typing undo group, but ordinary
		// cursor movement should.
		s.lastEdit = editorEditNone
	} else if !s.completionOn {
		s.lastEdit = editorEditNone
	}

	quit, err := s.handleUX(event)
	if err != nil {
		return quit, err
	}
	if mutates {
		s.lastEdit = class
		s.status = "autosaving…"
		s.statusError = false
		s.autosaver.Schedule(s.source())
	}
	return quit, nil
}

func (s *terminalEditorSession) editClass(event editorEvent) (editorEditClass, bool) {
	if s.completionOn && (event.key == editorKeyEnter || event.key == editorKeyTab) {
		return editorEditCompletion, len(s.completion.items) > 0
	}
	switch event.key {
	case editorKeyRune:
		return editorEditInsert, true
	case editorKeyBackspace:
		return editorEditBackspace, s.cursorX > 0 || s.cursorY > 0
	case editorKeyDelete:
		return editorEditDelete, s.cursorX < len(s.lines[s.cursorY]) || s.cursorY+1 < len(s.lines)
	case editorKeyEnter:
		return editorEditStructure, true
	case editorKeyTab:
		return editorEditIndent, true
	case editorKeyBacktab:
		line := s.lines[s.cursorY]
		return editorEditIndent, len(line) > 0 && line[0] == ' '
	default:
		return editorEditNone, false
	}
}

func (s *terminalEditorSession) recordBeforeEdit(class editorEditClass) {
	if s.lastEdit != class {
		s.undo.push(s.snapshot())
	}
	s.redo.clear()
}

func (s *terminalEditorSession) snapshot() editorSnapshot {
	return editorSnapshot{
		source:  s.source(),
		cursorX: s.cursorX,
		cursorY: s.cursorY,
		top:     s.top,
		left:    s.left,
	}
}

func (s *terminalEditorSession) restore(snapshot editorSnapshot) {
	normalized := snapshot.source
	if s.lineEnding == "\r\n" {
		normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	}
	parts := strings.Split(normalized, "\n")
	s.lines = make([][]rune, len(parts))
	for index, line := range parts {
		s.lines[index] = []rune(line)
	}
	if len(s.lines) == 0 {
		s.lines = [][]rune{{}}
	}
	s.cursorX = snapshot.cursorX
	s.cursorY = snapshot.cursorY
	s.top = snapshot.top
	s.left = snapshot.left
	s.ensureCursor()
	s.hideCompletion()
	s.dirty = sha256.Sum256([]byte(s.source())) != s.baseRevision
	s.status = "modified"
	s.statusError = false
}

func (s *terminalEditorSession) undoEdit() {
	snapshot, ok := s.undo.pop()
	if !ok {
		s.status = "nothing to undo"
		s.statusError = false
		return
	}
	s.redo.push(s.snapshot())
	s.restore(snapshot)
	s.lastEdit = editorEditNone
	s.status = "undo · autosaving…"
	s.autosaver.Schedule(s.source())
}

func (s *terminalEditorSession) redoEdit() {
	snapshot, ok := s.redo.pop()
	if !ok {
		s.status = "nothing to redo"
		s.statusError = false
		return
	}
	s.undo.push(s.snapshot())
	s.restore(snapshot)
	s.lastEdit = editorEditNone
	s.status = "redo · autosaving…"
	s.autosaver.Schedule(s.source())
}

func (s *editorSnapshotStack) push(snapshot editorSnapshot) {
	s.items = append(s.items, snapshot)
	s.bytes += len(snapshot.source)
	for len(s.items) > editorHistoryMaxStates || s.bytes > editorHistoryMaxBytes {
		s.bytes -= len(s.items[0].source)
		s.items = s.items[1:]
	}
}

func (s *editorSnapshotStack) pop() (editorSnapshot, bool) {
	if len(s.items) == 0 {
		return editorSnapshot{}, false
	}
	index := len(s.items) - 1
	snapshot := s.items[index]
	s.items = s.items[:index]
	s.bytes -= len(snapshot.source)
	return snapshot, true
}

func (s *editorSnapshotStack) clear() {
	s.items = nil
	s.bytes = 0
}

func (s *terminalEditorSession) openSearch() {
	s.hideCompletion()
	s.search.active = true
	s.search.query = nil
	s.search.matches = nil
	s.search.index = 0
	s.lastEdit = editorEditNone
}

func (s *terminalEditorSession) closeSearch() {
	s.search = editorSearchState{}
	s.lastEdit = editorEditNone
}

func (s *terminalEditorSession) handleSearch(input editorSessionEvent) (bool, error) {
	if input.special == editorSessionFind {
		s.closeSearch()
		return false, nil
	}
	if input.special != editorSessionNone {
		return false, nil
	}

	switch input.event.key {
	case editorKeyEscape:
		s.closeSearch()
	case editorKeyRune:
		s.search.query = append(s.search.query, input.event.r)
		s.refreshSearch(true)
	case editorKeyBackspace:
		if len(s.search.query) > 0 {
			s.search.query = s.search.query[:len(s.search.query)-1]
			s.refreshSearch(true)
		}
	case editorKeyEnter, editorKeyDown:
		s.moveSearch(1)
	case editorKeyUp:
		s.moveSearch(-1)
	}
	return false, nil
}

func (s *terminalEditorSession) refreshSearch(fromCursor bool) {
	s.search.matches = editorFindMatches(s.lines, s.search.query)
	if len(s.search.matches) == 0 {
		s.search.index = 0
		return
	}
	if !fromCursor {
		s.search.index = min(s.search.index, len(s.search.matches)-1)
		s.selectSearchMatch()
		return
	}
	s.search.index = 0
	for index, match := range s.search.matches {
		if match.line > s.cursorY || match.line == s.cursorY && match.column >= s.cursorX {
			s.search.index = index
			break
		}
	}
	s.selectSearchMatch()
}

func (s *terminalEditorSession) moveSearch(delta int) {
	if len(s.search.matches) == 0 {
		return
	}
	s.search.index = (s.search.index + delta + len(s.search.matches)) % len(s.search.matches)
	s.selectSearchMatch()
}

func (s *terminalEditorSession) selectSearchMatch() {
	if len(s.search.matches) == 0 {
		return
	}
	match := s.search.matches[s.search.index]
	s.cursorY = match.line
	s.cursorX = match.column
	s.ensureCursor()
}

func editorFindMatches(lines [][]rune, query []rune) []editorSearchMatch {
	if len(query) == 0 {
		return nil
	}
	needle := make([]rune, len(query))
	for index, r := range query {
		needle[index] = unicode.ToLower(r)
	}
	matches := make([]editorSearchMatch, 0)
	for lineIndex, line := range lines {
		for column := 0; column+len(needle) <= len(line); column++ {
			matched := true
			for offset, want := range needle {
				if unicode.ToLower(line[column+offset]) != want {
					matched = false
					break
				}
			}
			if matched {
				matches = append(matches, editorSearchMatch{line: lineIndex, column: column, length: len(needle)})
				column += len(needle) - 1
			}
		}
	}
	return matches
}

func (s *terminalEditorSession) drawSearchOverlay() {
	width, height := terminalSize(os.Stdout)
	width = max(width, 40)
	height = max(height, 10)
	palette := s.terminal.colors()

	count := "0/0"
	if len(s.search.matches) > 0 {
		count = fmt.Sprintf("%d/%d", s.search.index+1, len(s.search.matches))
	}
	query := safeTerminalText(string(s.search.query))
	if query == "" {
		query = "type to search"
	}
	plain := " Find  " + query + "  " + count + " · Enter/↓ next · ↑ previous · Esc close"
	bar := s.terminal.paint(ansiBold+palette.accent, terminalClip(plain, width))
	fmt.Fprintf(s.terminal.out, "\x1b[%d;1H\x1b[2K%s", height, bar)

	if len(s.search.matches) > 0 {
		match := s.search.matches[s.search.index]
		digits := len(fmt.Sprint(len(s.lines)))
		gutterWidth := digits + 3
		codeWidth := max(1, width-gutterWidth)
		row := 2 + (match.line - s.top)
		start := match.column
		end := match.column + match.length
		if row >= 2 && row <= height-2 && end > s.left && start < s.left+codeWidth {
			visibleStart := max(start, s.left)
			visibleEnd := min(end, s.left+codeWidth)
			column := gutterWidth + 1 + (visibleStart - s.left)
			text := string(s.lines[match.line][visibleStart:visibleEnd])
			fmt.Fprintf(s.terminal.out, "\x1b[%d;%dH%s", row, column, s.terminal.paint(ansiBold+palette.accent, text))
		}
	}

	cursorRow := 2 + (s.cursorY - s.top)
	digits := len(fmt.Sprint(len(s.lines)))
	cursorColumn := digits + 4 + (s.cursorX - s.left)
	cursorColumn = min(max(digits+4, cursorColumn), width)
	fmt.Fprintf(s.terminal.out, "\x1b[%d;%dH\x1b[?25h", cursorRow, cursorColumn)
}

func (s *terminalEditorSession) saveNow() error {
	result := s.autosaver.SaveNow(s.source())
	s.applyAutosaveResult(result)
	return result.err
}

func (s *terminalEditorSession) pollAutosave() {
	if s.autosaver == nil {
		return
	}
	for {
		select {
		case result := <-s.autosaver.results:
			s.applyAutosaveResult(result)
		default:
			return
		}
	}
}

func (s *terminalEditorSession) applyAutosaveResult(result editorAutosaveResult) {
	if result.err != nil {
		s.status = "autosave blocked · " + result.err.Error()
		s.statusError = true
		return
	}
	s.baseRevision = result.revision
	if sha256.Sum256([]byte(s.source())) == result.sourceHash {
		s.dirty = false
		s.status = "saved"
	} else {
		s.dirty = true
		s.status = "autosaving…"
	}
	s.statusError = false
}

func (s *terminalEditorSession) escapeEditor() (bool, error) {
	s.pollAutosave()
	if !s.dirty {
		return true, nil
	}
	if s.confirmQuit {
		return true, nil
	}
	if err := s.saveNow(); err == nil {
		return true, nil
	}
	s.confirmQuit = true
	s.status = "save conflict · press Esc again to discard local changes"
	s.statusError = true
	return false, nil
}

func newEditorAutosaver(path string, revision [sha256.Size]byte, delay time.Duration) *editorAutosaver {
	a := &editorAutosaver{
		path:     path,
		delay:    delay,
		commands: make(chan editorAutosaveCommand, 64),
		results:  make(chan editorAutosaveResult, 16),
		stop:     make(chan struct{}),
	}
	go a.run(revision)
	return a
}

func (a *editorAutosaver) Schedule(source string) {
	if a == nil || a.stopped {
		return
	}
	a.commands <- editorAutosaveCommand{source: source}
}

func (a *editorAutosaver) SaveNow(source string) editorAutosaveResult {
	if a == nil || a.stopped {
		return editorAutosaveResult{err: errors.New("autosave is unavailable")}
	}
	response := make(chan editorAutosaveResult, 1)
	a.commands <- editorAutosaveCommand{source: source, force: true, response: response}
	return <-response
}

func (a *editorAutosaver) Stop() {
	if a == nil || a.stopped {
		return
	}
	a.stopped = true
	close(a.stop)
}

func (a *editorAutosaver) run(revision [sha256.Size]byte) {
	var pending string
	var hasPending bool
	var timer *time.Timer
	var timerC <-chan time.Time

	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timerC = nil
	}
	resetTimer := func() {
		if timer == nil {
			timer = time.NewTimer(a.delay)
		} else {
			stopTimer()
			timer.Reset(a.delay)
		}
		timerC = timer.C
	}
	write := func(source string) editorAutosaveResult {
		data := []byte(source)
		result := editorAutosaveResult{sourceHash: sha256.Sum256(data)}
		if len(data) > 2*1024*1024 {
			result.err = errors.New("document exceeds 2 MiB limit")
			return result
		}
		current, err := os.ReadFile(a.path)
		if err != nil {
			result.err = err
			return result
		}
		if sha256.Sum256(current) != revision {
			result.err = errors.New("source changed on disk; reopen before saving")
			return result
		}
		if result.sourceHash == revision {
			result.revision = revision
			return result
		}
		info, err := os.Stat(a.path)
		if err != nil {
			result.err = err
			return result
		}
		if err := os.WriteFile(a.path, data, info.Mode().Perm()); err != nil {
			result.err = err
			return result
		}
		revision = result.sourceHash
		result.revision = revision
		return result
	}

	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-a.stop:
			return
		case command := <-a.commands:
			if command.force {
				stopTimer()
				pending = ""
				hasPending = false
				command.response <- write(command.source)
				continue
			}
			pending = command.source
			hasPending = true
			resetTimer()
		case <-timerC:
			timerC = nil
			if !hasPending {
				continue
			}
			result := write(pending)
			pending = ""
			hasPending = false
			a.results <- result
		}
	}
}
