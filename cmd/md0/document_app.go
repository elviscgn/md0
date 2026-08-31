package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	core "github.com/elviscgn/md0/internal/md0"
)

const (
	documentAppViewerHost      = "127.0.0.1"
	documentAppViewerPortStart = 8080
	documentAppViewerPortEnd   = 8099
)

type documentApp struct {
	path          string
	reader        *bufio.Reader
	selected      int
	status        string
	statusError   bool
	viewerStarted bool
	viewerAddr    string
	viewerErr     chan error
}

func runDocumentApp(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	if !launcherAvailable() {
		return errors.New("interactive document app requires a terminal; use an explicit md0 subcommand")
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return errors.New("interactive document app is unavailable when TERM=dumb")
	}

	restore, err := enableRawTerminal(os.Stdin)
	if err != nil {
		return fmt.Errorf("document app could not enable interactive input: %w", err)
	}
	fmt.Fprint(cliUI.out, "\x1b[?1049h\x1b[2J\x1b[H\x1b[?25l")
	defer func() {
		_ = restore()
		fmt.Fprint(cliUI.out, "\x1b[2J\x1b[H\x1b[?25h\x1b[?1049l")
	}()

	app := &documentApp{
		path:      path,
		reader:    bufio.NewReader(os.Stdin),
		viewerErr: make(chan error, 1),
	}
	return app.run()
}

func (a *documentApp) run() error {
	for {
		a.pollViewerError()
		a.drawHome()
		key, err := readLauncherKey(a.reader)
		if err != nil {
			return err
		}
		switch key {
		case launcherKeyUp:
			a.selected = (a.selected - 1 + len(launcherOptions)) % len(launcherOptions)
		case launcherKeyDown:
			a.selected = (a.selected + 1) % len(launcherOptions)
		case launcherKeySelect:
			quit, err := a.activate(launcherOptions[a.selected].action)
			if err != nil {
				a.status = err.Error()
				a.statusError = true
			}
			if quit {
				return nil
			}
		case launcherKeyQuit:
			return nil
		case launcherKeyHelp:
			if err := a.showHelp(); err != nil {
				a.status = err.Error()
				a.statusError = true
			}
		default:
			if action, ok := launcherActionForKey(key); ok {
				quit, err := a.activate(action)
				if err != nil {
					a.status = err.Error()
					a.statusError = true
				}
				if quit {
					return nil
				}
			}
		}
	}
}

func (a *documentApp) activate(action launcherAction) (bool, error) {
	a.status = ""
	a.statusError = false
	switch action {
	case launcherEdit:
		return false, a.runEditor()
	case launcherOpen:
		return false, a.openViewer()
	case launcherRender:
		return false, a.renderHTML()
	case launcherInspect:
		return false, a.inspectDocument()
	case launcherValidate:
		return false, a.validateDocument()
	case launcherQuit:
		return true, nil
	default:
		return false, nil
	}
}

func (a *documentApp) drawHome() {
	view := cliUI.launcherView(a.path, a.selected, true)
	var out strings.Builder
	out.WriteString("\x1b[2J\x1b[H\x1b[?25l")
	out.WriteString(view)
	width, height := terminalSize(os.Stdout)
	width = max(width, 40)
	height = max(height, 10)
	status := " ? help · Esc/q exits md0"
	style := ansiDim
	if a.viewerStarted {
		status = " ● viewer live · http://" + a.viewerAddr + " · ? help · Esc/q exits md0"
	}
	if a.status != "" {
		status = " " + a.status
		if a.statusError {
			style = ansiBold + cliUI.colors().error
		} else {
			style = ansiBold + cliUI.colors().success
		}
	}
	fmt.Fprintf(&out, "\x1b[%d;1H\x1b[2K%s", height, cliUI.paint(style, terminalClip(status, width)))
	_, _ = fmt.Fprint(cliUI.out, out.String())
}

func (a *documentApp) runEditor() error {
	data, err := os.ReadFile(a.path)
	if err != nil {
		return err
	}
	if !utf8.Valid(data) {
		return errors.New("document must be valid UTF-8")
	}
	if len(data) > 2*1024*1024 {
		return errors.New("document exceeds 2 MiB limit")
	}
	editor := newTerminalEditorSession(a.path, string(data), newTerminalUI(os.Stdout))
	defer editor.close()
	for {
		editor.draw("back")
		event, err := readEditorSessionEvent(a.reader)
		if err != nil {
			return err
		}
		back, err := editor.handle(event)
		if err != nil {
			editor.status = err.Error()
			editor.statusError = true
		}
		if back {
			fmt.Fprint(cliUI.out, "\x1b[2J\x1b[H")
			return nil
		}
	}
}

func (a *documentApp) openViewer() error {
	if a.viewerStarted {
		scheduleBrowserOpen(a.viewerAddr, false)
		a.status = "viewer opened · http://" + a.viewerAddr
		return nil
	}
	doc, err := core.ParseFile(a.path)
	if err != nil {
		return fmt.Errorf("cannot open viewer: %s", core.FormatDiagnostic(a.path, err))
	}
	if _, err := core.Evaluate(doc, nil); err != nil {
		return fmt.Errorf("cannot open viewer: %s", core.FormatDiagnostic(a.path, err))
	}
	addr, err := nextDocumentViewerAddr()
	if err != nil {
		return err
	}

	a.viewerStarted = true
	a.viewerAddr = addr
	go func() {
		err := core.ServeFileWorkspaceWithOptions(a.path, addr, nil, nil)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.viewerErr <- err
		}
	}()
	scheduleBrowserOpen(addr, false)
	a.status = "viewer started · http://" + addr
	return nil
}

func nextDocumentViewerAddr() (string, error) {
	for port := documentAppViewerPortStart; port <= documentAppViewerPortEnd; port++ {
		addr := fmt.Sprintf("%s:%d", documentAppViewerHost, port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = listener.Close()
		return addr, nil
	}
	return "", fmt.Errorf("no free loopback viewer port between %d and %d", documentAppViewerPortStart, documentAppViewerPortEnd)
}

func (a *documentApp) pollViewerError() {
	select {
	case err := <-a.viewerErr:
		a.viewerStarted = false
		a.viewerAddr = ""
		a.status = "viewer stopped: " + err.Error()
		a.statusError = true
	default:
	}
}

func (a *documentApp) renderHTML() error {
	doc, err := core.ParseFile(a.path)
	if err != nil {
		return a.showDiagnostic("Render failed", err)
	}
	result, err := core.Evaluate(doc, nil)
	if err != nil {
		return a.showDiagnostic("Render failed", err)
	}
	fragment, err := core.RenderFragmentBounded(doc, result)
	if err != nil {
		return a.showDiagnostic("Render failed", err)
	}
	out := defaultHTMLPath(a.path)
	if err := os.WriteFile(out, []byte(core.RenderStaticPage(doc.Path, fragment)), 0644); err != nil {
		return err
	}
	a.status = "rendered · " + out
	return nil
}

func (a *documentApp) validateDocument() error {
	doc, err := core.ParseFile(a.path)
	if err != nil {
		return a.showDiagnostic("Validation failed", err)
	}
	result, err := core.Evaluate(doc, nil)
	if err != nil {
		return a.showDiagnostic("Validation failed", err)
	}
	for _, assertion := range result.Assertions {
		if !assertion.Passed {
			message := fmt.Sprintf("line %d: %s", assertion.Line, assertion.Source)
			if assertion.Message != "" {
				message += " — " + assertion.Message
			}
			return a.showText("Validation failed", message)
		}
	}
	a.status = "valid md0/PURE document"
	return nil
}

func (a *documentApp) inspectDocument() error {
	doc, err := core.ParseFile(a.path)
	if err != nil {
		return a.showDiagnostic("Inspect failed", err)
	}
	return a.showText("Inspect", core.Inspect(doc))
}

func (a *documentApp) showHelp() error {
	const help = `HOME
  ↑/↓ or j/k   move selection
  Enter        open selected action
  e            edit document
  o            open/reopen live viewer
  r            render standalone HTML
  i            inspect dependency graph + authority
  v            validate document
  ?            show this help
  Esc / q      exit md0

EDITOR
  Esc          save pending changes and return home
  Ctrl+S       save immediately
  Ctrl+Z       undo
  Ctrl+Y       redo
  Ctrl+F       find in document
  Ctrl+Space   completion
  Tab          indent / accept completion
  Shift+Tab    outdent
  ↑/↓          cursor / completion navigation

LIVE EDITING
  Changes autosave after a short pause. If the browser viewer is open,
  its existing source watcher sees those saves and refreshes the document.
  External disk edits are protected by revision checks instead of overwritten.

The md0/PURE document language still has no ambient filesystem, network,
process, environment, package, native-code, or dynamic-eval authority.`
	return a.showText("Help", help)
}

func (a *documentApp) showDiagnostic(title string, err error) error {
	return a.showText(title, core.FormatDiagnostic(a.path, err))
}

func (a *documentApp) showText(title, body string) error {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	top := 0
	for {
		width, height := terminalSize(os.Stdout)
		width = max(width, 40)
		height = max(height, 10)
		rows := max(1, height-3)
		if top > max(0, len(lines)-rows) {
			top = max(0, len(lines)-rows)
		}
		var out strings.Builder
		out.WriteString("\x1b[2J\x1b[H\x1b[?25l")
		header := " md0/PURE · " + title
		fmt.Fprintf(&out, "\x1b[1;1H%s", cliUI.paint(ansiBold+cliUI.colors().accent, terminalClip(header, width)))
		for row := 0; row < rows; row++ {
			index := top + row
			if index >= len(lines) {
				break
			}
			fmt.Fprintf(&out, "\x1b[%d;1H%s", 2+row, terminalClip(safeTerminalText(lines[index]), width))
		}
		footer := " Esc/Enter back · ↑↓ scroll · PgUp/PgDn page"
		fmt.Fprintf(&out, "\x1b[%d;1H%s", height, cliUI.paint(ansiDim, terminalClip(footer, width)))
		_, _ = fmt.Fprint(cliUI.out, out.String())

		event, err := readEditorEvent(a.reader)
		if err != nil {
			return err
		}
		switch event.key {
		case editorKeyEscape, editorKeyEnter, editorKeyQuit:
			return nil
		case editorKeyUp:
			top = max(0, top-1)
		case editorKeyDown:
			top = min(max(0, len(lines)-rows), top+1)
		case editorKeyPageUp:
			top = max(0, top-rows)
		case editorKeyPageDown:
			top = min(max(0, len(lines)-rows), top+rows)
		}
	}
}
