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
		fmt.Println("md0 dev")
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}
func usage() {
	fmt.Fprint(os.Stderr, `md0 — safe computational Markdown

Usage:
  md0 validate FILE
  md0 eval FILE
  md0 render [-o FILE] FILE
  md0 open [-addr 127.0.0.1:8080] FILE
  md0 inspect FILE
`)
}
func oneFile(name string, args []string) *core.Document {
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "%s expects exactly one file\n", name)
		os.Exit(2)
	}
	doc, err := core.ParseFile(args[0])
	if err != nil {
		die(err)
	}
	return doc
}
func cmdValidate(args []string) {
	doc := oneFile("validate", args)
	r, err := core.Evaluate(doc, nil)
	if err != nil {
		die(err)
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
	doc := oneFile("eval", args)
	r, err := core.Evaluate(doc, nil)
	if err != nil {
		die(err)
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
	fs.Parse(args)
	doc := oneFile("render", fs.Args())
	r, err := core.Evaluate(doc, nil)
	if err != nil {
		die(err)
	}
	frag, err := core.RenderFragment(doc, r)
	if err != nil {
		die(err)
	}
	page := core.RenderStaticPage(doc.Path, frag)
	if *out == "" {
		fmt.Print(page)
		return
	}
	if err := os.WriteFile(*out, []byte(page), 0644); err != nil {
		die(err)
	}
	fmt.Printf("wrote %s\n", *out)
}
func cmdOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	fs.Parse(args)
	doc := oneFile("open", fs.Args())
	if _, err := core.Evaluate(doc, nil); err != nil {
		die(err)
	}
	if err := core.Serve(doc, *addr); err != nil {
		die(err)
	}
}
func cmdInspect(args []string) { doc := oneFile("inspect", args); fmt.Print(core.Inspect(doc)) }
func die(err error)            { fmt.Fprintln(os.Stderr, "md0:", err); os.Exit(1) }
