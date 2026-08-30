package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	core "github.com/elviscgn/md0/internal/md0"
)

func looksLikeDocumentArg(arg string) bool {
	if strings.HasSuffix(strings.ToLower(arg), ".md") || strings.HasSuffix(strings.ToLower(arg), ".md0") {
		return true
	}
	info, err := os.Stat(arg)
	return err == nil && !info.IsDir()
}

func launcherAvailable() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func launchDocument(path string) {
	if _, err := os.Stat(path); err != nil {
		die(err)
	}
	if !launcherAvailable() {
		cliError("interactive document launcher requires a terminal; use an explicit md0 subcommand")
		os.Exit(2)
	}

	cliUI.logo()
	fmt.Fprintln(cliUI.out)
	cliUI.meta("document", path)
	cliUI.meta("runtime", "md0/PURE")
	fmt.Fprintln(cliUI.out)
	cliUI.choice("e", "Edit + live preview", "authoring mode", true)
	cliUI.choice("o", "Open live viewer", "watch source", false)
	cliUI.choice("r", "Render standalone HTML", "offline output", false)
	cliUI.choice("i", "Inspect document", "graph + authority", false)
	cliUI.choice("v", "Validate", "parse + evaluate", false)
	cliUI.choice("q", "Quit", "", false)
	fmt.Fprintln(cliUI.out)
	fmt.Fprint(cliUI.out, cliUI.paint(ansiDim, "Choose an action: "))

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		die(err)
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" {
		answer = "e"
	}

	switch answer {
	case "e", "1", "edit":
		cmdEdit([]string{path})
	case "o", "2", "open":
		cmdOpen([]string{path})
	case "r", "3", "render":
		out := defaultHTMLPath(path)
		cmdRender([]string{"-o", out, path})
	case "i", "4", "inspect":
		cmdInspect([]string{path})
	case "v", "5", "validate":
		cmdValidate([]string{path})
	case "q", "6", "quit":
		return
	default:
		cliError(fmt.Sprintf("unknown action %q", answer))
		os.Exit(2)
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
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "loopback listen address")
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	data := addDataFlags(fs)
	fs.Parse(args)
	doc := oneFile("edit", fs.Args())
	bindData(doc, *data)
	values := loadValues(*valuesPath)
	if _, err := core.Evaluate(doc, values); err != nil {
		dieDoc(doc, err)
	}
	cliUI.command("edit")
	cliUI.meta("document", doc.Path)
	cliUI.meta("mode", "live authoring")
	cliUI.meta("watching", "enabled")
	fmt.Fprintln(cliUI.out)
	cliUI.success("parsed and evaluated")
	cliUI.action("http://" + *addr)
	fmt.Fprintln(cliUI.out, cliUI.paint(ansiDim, "  Cmd/Ctrl+S saves · Ctrl+C stops"))
	if err := core.ServeFileEditorWithOptions(doc.Path, *addr, values, *data); err != nil {
		die(err)
	}
}
