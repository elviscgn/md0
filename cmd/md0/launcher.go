package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type launcherAction int

const (
	launcherEdit launcherAction = iota
	launcherOpen
	launcherRender
	launcherInspect
	launcherValidate
	launcherQuit
)

type launcherOption struct {
	key    string
	label  string
	hint   string
	action launcherAction
}

var launcherOptions = []launcherOption{
	{key: "e", label: "Edit in terminal", hint: "syntax + completion", action: launcherEdit},
	{key: "o", label: "Open live viewer", hint: "watch source changes", action: launcherOpen},
	{key: "r", label: "Render standalone HTML", hint: "create an offline document", action: launcherRender},
	{key: "i", label: "Inspect document", hint: "dependencies + authority", action: launcherInspect},
	{key: "v", label: "Validate", hint: "parse + evaluate", action: launcherValidate},
	{key: "q", label: "Quit", hint: "return to the terminal", action: launcherQuit},
}

type launcherKey int

const (
	launcherKeyUnknown launcherKey = iota
	launcherKeyUp
	launcherKeyDown
	launcherKeySelect
	launcherKeyQuit
	launcherKeyEdit
	launcherKeyOpen
	launcherKeyRender
	launcherKeyInspect
	launcherKeyValidate
)

func looksLikeDocumentArg(arg string) bool {
	if strings.HasSuffix(strings.ToLower(arg), ".md") || strings.HasSuffix(strings.ToLower(arg), ".md0") {
		return true
	}
	info, err := os.Stat(arg)
	return err == nil && !info.IsDir()
}

func launcherAvailable() bool {
	input, inputErr := os.Stdin.Stat()
	output, outputErr := os.Stdout.Stat()
	return inputErr == nil && outputErr == nil &&
		input.Mode()&os.ModeCharDevice != 0 && output.Mode()&os.ModeCharDevice != 0
}

func launchDocument(path string) {
	if _, err := os.Stat(path); err != nil {
		die(err)
	}
	if !launcherAvailable() {
		cliError("interactive document launcher requires a terminal; use an explicit md0 subcommand")
		os.Exit(2)
	}

	action, err := chooseLauncherAction(path)
	if err != nil {
		die(err)
	}

	switch action {
	case launcherEdit:
		cmdEdit([]string{path})
	case launcherOpen:
		cmdOpen([]string{path})
	case launcherRender:
		out := defaultHTMLPath(path)
		cmdRender([]string{"-o", out, path})
	case launcherInspect:
		cmdInspect([]string{path})
	case launcherValidate:
		cmdValidate([]string{path})
	case launcherQuit:
		return
	}
}

func chooseLauncherAction(path string) (launcherAction, error) {
	if !strings.EqualFold(os.Getenv("TERM"), "dumb") {
		if restore, err := enableRawTerminal(os.Stdin); err == nil {
			return chooseLauncherActionRaw(path, restore)
		}
	}
	return chooseLauncherActionFallback(path, bufio.NewReader(os.Stdin))
}

func chooseLauncherActionRaw(path string, restore func() error) (launcherAction, error) {
	reader := bufio.NewReader(os.Stdin)
	selected := 0
	view := cliUI.launcherView(path, selected, true)

	fmt.Fprint(cliUI.out, "\x1b[2J\x1b[H\x1b[?25l", view)
	cleaned := false
	cleanup := func() error {
		if cleaned {
			return nil
		}
		cleaned = true
		err := restore()
		fmt.Fprint(cliUI.out, "\x1b[2J\x1b[H\x1b[?25h")
		return err
	}
	defer func() { _ = cleanup() }()

	for {
		key, err := readLauncherKey(reader)
		if err != nil {
			_ = cleanup()
			return launcherQuit, err
		}
		switch key {
		case launcherKeyUp:
			selected = (selected - 1 + len(launcherOptions)) % len(launcherOptions)
		case launcherKeyDown:
			selected = (selected + 1) % len(launcherOptions)
		case launcherKeySelect:
			action := launcherOptions[selected].action
			return action, cleanup()
		case launcherKeyQuit:
			return launcherQuit, cleanup()
		default:
			if action, ok := launcherActionForKey(key); ok {
				return action, cleanup()
			}
			continue
		}

		fmt.Fprintf(cliUI.out, "\x1b[H\x1b[J%s", cliUI.launcherView(path, selected, true))
	}
}

func chooseLauncherActionFallback(path string, reader *bufio.Reader) (launcherAction, error) {
	selected := 0
	for {
		if !strings.EqualFold(os.Getenv("TERM"), "dumb") {
			fmt.Fprint(cliUI.out, "\x1b[2J\x1b[H")
		}
		fmt.Fprint(cliUI.out, cliUI.launcherView(path, selected, false))
		fmt.Fprint(cliUI.out, cliUI.paint(ansiDim, "Choose an action: "))
		answer, err := reader.ReadString('\n')
		if err != nil {
			return launcherQuit, err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer == "" {
			return launcherOptions[selected].action, nil
		}
		if action, ok := launcherActionForAnswer(answer); ok {
			return action, nil
		}
		cliUI.warning("Use e, o, r, i, v, or q, then press Enter.")
		fmt.Fprintln(cliUI.out)
	}
}

func readLauncherKey(reader *bufio.Reader) (launcherKey, error) {
	value, err := reader.ReadByte()
	if err != nil {
		return launcherKeyUnknown, err
	}
	switch value {
	case '\r', '\n':
		return launcherKeySelect, nil
	case 3:
		return launcherKeyQuit, nil
	case 'k', 'K':
		return launcherKeyUp, nil
	case 'j', 'J', '\t':
		return launcherKeyDown, nil
	case 'e', 'E':
		return launcherKeyEdit, nil
	case 'o', 'O':
		return launcherKeyOpen, nil
	case 'r', 'R':
		return launcherKeyRender, nil
	case 'i', 'I':
		return launcherKeyInspect, nil
	case 'v', 'V':
		return launcherKeyValidate, nil
	case 'q', 'Q':
		return launcherKeyQuit, nil
	case 0x1b:
		return readLauncherEscape(reader)
	default:
		return launcherKeyUnknown, nil
	}
}

func readLauncherEscape(reader *bufio.Reader) (launcherKey, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return launcherKeyUnknown, err
	}
	if prefix != '[' && prefix != 'O' {
		return launcherKeyUnknown, nil
	}
	for range 8 {
		value, err := reader.ReadByte()
		if err != nil {
			return launcherKeyUnknown, err
		}
		switch value {
		case 'A':
			return launcherKeyUp, nil
		case 'B':
			return launcherKeyDown, nil
		case 'C', 'D', 'H', 'F', '~':
			return launcherKeyUnknown, nil
		}
	}
	return launcherKeyUnknown, nil
}

func launcherActionForKey(key launcherKey) (launcherAction, bool) {
	switch key {
	case launcherKeyEdit:
		return launcherEdit, true
	case launcherKeyOpen:
		return launcherOpen, true
	case launcherKeyRender:
		return launcherRender, true
	case launcherKeyInspect:
		return launcherInspect, true
	case launcherKeyValidate:
		return launcherValidate, true
	default:
		return launcherQuit, false
	}
}

func launcherActionForAnswer(answer string) (launcherAction, bool) {
	switch answer {
	case "e", "1", "edit":
		return launcherEdit, true
	case "o", "2", "open":
		return launcherOpen, true
	case "r", "3", "render":
		return launcherRender, true
	case "i", "4", "inspect":
		return launcherInspect, true
	case "v", "5", "validate":
		return launcherValidate, true
	case "q", "6", "quit":
		return launcherQuit, true
	default:
		return launcherQuit, false
	}
}

func defaultHTMLPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path + ".html"
	}
	return strings.TrimSuffix(path, ext) + ".html"
}

func cmdEdit(args []string) {
	if len(args) != 1 {
		cliError("edit expects exactly one file")
		os.Exit(2)
	}
	if err := runTerminalEditor(args[0]); err != nil {
		die(err)
	}
}
