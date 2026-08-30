package main

import (
	"flag"
	"fmt"
	"os"

	core "github.com/elviscgn/md0/internal/md0"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
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
	case "inspect":
		cmdInspect(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("md0", core.RuntimeVersion)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `md0 — safe computational Markdown

Usage:
  md0 validate [--values FILE] FILE
  md0 eval [--values FILE] FILE
  md0 render [-o FILE] [--values FILE] [--snapshot FILE] FILE
  md0 open [-addr 127.0.0.1:8080] [--values FILE] FILE
  md0 inspect FILE
`)
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
		fmt.Fprintf(os.Stderr, "%s expects exactly one file\n", name)
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
	fs.Parse(args)
	doc := oneFile("validate", fs.Args())
	r, err := core.Evaluate(doc, loadValues(*valuesPath))
	if err != nil {
		dieDoc(doc, err)
	}
	failed := false
	for _, a := range r.Assertions {
		if !a.Passed {
			failed = true
			fmt.Fprintf(os.Stderr, "FAIL line %d: %s", a.Line, a.Source)
			if a.Message != "" {
				fmt.Fprintf(os.Stderr, " — %s", a.Message)
			}
			fmt.Fprintln(os.Stderr)
		}
	}
	if failed {
		os.Exit(1)
	}
	fmt.Printf("ok — %s is valid md0/PURE\n", doc.Path)
}

func cmdEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	fs.Parse(args)
	doc := oneFile("eval", fs.Args())
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
	fs.Parse(args)
	doc := oneFile("render", fs.Args())
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
	fmt.Printf("wrote %s\n", *out)
	if *snapshotPath != "" {
		fmt.Printf("wrote %s\n", *snapshotPath)
	}
}

func cmdOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "loopback listen address")
	valuesPath := fs.String("values", "", "JSON values or md0 snapshot file")
	fs.Parse(args)
	doc := oneFile("open", fs.Args())
	values := loadValues(*valuesPath)
	if _, err := core.Evaluate(doc, values); err != nil {
		dieDoc(doc, err)
	}
	if err := core.ServeWithValues(doc, *addr, values); err != nil {
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
	fmt.Fprintln(os.Stderr, "md0:", core.FormatDiagnostic(path, err))
	os.Exit(1)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "md0:", err)
	os.Exit(1)
}
