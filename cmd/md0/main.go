package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	core "github.com/elviscgn/md0/internal/md0"
)

func main() {
	if len(os.Args) < 2 {
		cliUI.logo()
		fmt.Fprintln(os.Stderr)
		usage()
		os.Exit(2)
	}
	if looksLikeDocumentArg(os.Args[1]) {
		launchDocument(os.Args[1])
		return
	}
	switch os.Args[1] {
	case "validate":
		cmdValidate(os.Args[2:])
	case "eval":
		cmdEval(os.Args[2:])
	case "render":
		cmdRender(os.Args[2:])
	case "open":
		cmdOpen(os.Args[2:])
	case "edit":
		cmdEdit(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("md0", core.RuntimeVersion)
	default:
		cliError(fmt.Sprintf("unknown command %q", os.Args[1]))
		fmt.Fprintln(os.Stderr)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `md0 — safe computational Markdown

Usage:
  md0 FILE
  md0 edit FILE
  md0 open [-addr 127.0.0.1:8080] [--no-browser] [--values FILE] [--data NAME=FILE] FILE
  md0 validate [--values FILE] [--data NAME=FILE] FILE
  md0 eval [--values FILE] [--data NAME=FILE] FILE
  md0 render [-o FILE] [--values FILE] [--data NAME=FILE] [--snapshot FILE] FILE
  md0 inspect FILE

Run md0 FILE in a terminal for the interactive document launcher.
`)
}

type dataFlags []string

func (f *dataFlags) String() string { return strings.Join(*f, ",") }

func (f *dataFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func addDataFlags(fs *flag.FlagSet) *dataFlags {
	var values dataFlags
	fs.Var(&values, "data", "bind declared data as NAME=FILE (repeatable)")
	return &values
}

func bindData(doc *core.Document, specs []string) {
	if err := core.BindDataFiles(doc, specs); err != nil {
		dieDoc(doc, err)
	}
}

func loadValues(path string) map[string]string {
	if path == "" {
		return nil
	}
	values, err := core.LoadValuesFile(path)
	if err != nil {
		die(err)
	}
	return values
}

func oneFile(name string, args []string) *core.Document {
	if len(args) != 1 {
		cliError(fmt.Sprintf("%s expects exactly one file", name))
		os.Exit(2)
	}
	doc, err := core.ParseFile(args[0])
	if err != nil {
		dieSource(args[0], err)
	}
	return doc
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	data := addDataFlags(fs)
	fs.Parse(args)
	doc := oneFile("validate", fs.Args())
	bindData(doc, *data)
	r, err := core.Evaluate(doc, loadValues(*valuesPath))
	if err != nil {
		dieDoc(doc, err)
	}
	failed := false
	for _, a := range r.Assertions {
		if !a.Passed {
			failed = true
			message := fmt.Sprintf("line %d: %s", a.Line, a.Source)
			if a.Message != "" {
				message += " — " + a.Message
			}
			newTerminalUI(os.Stderr).fail(message)
		}
	}
	if failed {
		os.Exit(1)
	}
	cliUI.command("validate")
	cliUI.meta("document", doc.Path)
	fmt.Fprintln(cliUI.out)
	cliUI.success("valid md0/PURE document")
}

func cmdEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	data := addDataFlags(fs)
	fs.Parse(args)
	doc := oneFile("eval", fs.Args())
	bindData(doc, *data)
	r, err := core.Evaluate(doc, loadValues(*valuesPath))
	if err != nil {
		dieDoc(doc, err)
	}
	for _, k := range core.SortedEnv(r.Env) {
		fmt.Printf("%-20s %s\n", k, r.Env[k].String())
	}
	if len(r.Assertions) > 0 {
		fmt.Println()
		for _, a := range r.Assertions {
			s := "PASS"
			if !a.Passed {
				s = "FAIL"
			}
			fmt.Printf("%-4s line %-4d %s\n", s, a.Line, a.Source)
		}
	}
}

func cmdRender(args []string) {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	out := fs.String("o", "", "output HTML file (default stdout)")
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	snapshotPath := fs.String("snapshot", "", "write durable snapshot JSON")
	data := addDataFlags(fs)
	fs.Parse(args)
	doc := oneFile("render", fs.Args())
	bindData(doc, *data)
	r, err := core.Evaluate(doc, loadValues(*valuesPath))
	if err != nil {
		dieDoc(doc, err)
	}
	frag, err := core.RenderFragmentBounded(doc, r)
	if err != nil {
		dieDoc(doc, err)
	}
	page := core.RenderStaticPage(doc.Path, frag)
	if *snapshotPath != "" {
		data, err := core.MarshalSnapshot(doc, r)
		if err != nil {
			dieDoc(doc, err)
		}
		if err := os.WriteFile(*snapshotPath, data, 0644); err != nil {
			die(err)
		}
	}
	if *out == "" {
		fmt.Print(page)
		if *snapshotPath != "" {
			fmt.Fprintf(os.Stderr, "wrote %s\n", *snapshotPath)
		}
		return
	}
	if err := os.WriteFile(*out, []byte(page), 0644); err != nil {
		die(err)
	}
	cliUI.command("render")
	cliUI.meta("document", doc.Path)
	fmt.Fprintln(cliUI.out)
	cliUI.success("rendered standalone HTML")
	cliUI.action(*out)
	if *snapshotPath != "" {
		cliUI.success("wrote durable snapshot")
		cliUI.action(*snapshotPath)
	}
}

func cmdOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "loopback listen address")
	noBrowser := fs.Bool("no-browser", false, "do not open the browser automatically")
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	data := addDataFlags(fs)
	fs.Parse(args)
	doc := oneFile("open", fs.Args())
	bindData(doc, *data)
	values := loadValues(*valuesPath)
	if _, err := core.Evaluate(doc, values); err != nil {
		dieDoc(doc, err)
	}
	cliUI.command("open")
	cliUI.meta("document", doc.Path)
	cliUI.meta("runtime", "PURE")
	cliUI.meta("watching", "enabled")
	fmt.Fprintln(cliUI.out)
	cliUI.success("parsed and evaluated")
	cliUI.action("http://" + *addr)
	fmt.Fprintln(cliUI.out, cliUI.paint(ansiDim, "  Ctrl+C to stop"))
	scheduleBrowserOpen(*addr, *noBrowser)
	if err := core.ServeFileWorkspaceWithOptions(doc.Path, *addr, values, *data); err != nil {
		die(err)
	}
}

func cmdInspect(args []string) {
	doc := oneFile("inspect", args)
	fmt.Print(core.Inspect(doc))
}

func dieDoc(doc *core.Document, err error) {
	dieSource(doc.Path, err)
}

func dieSource(path string, err error) {
	newTerminalUI(os.Stderr).fail(core.FormatDiagnostic(path, err))
	os.Exit(1)
}

func die(err error) {
	cliError(err.Error())
	os.Exit(1)
}
