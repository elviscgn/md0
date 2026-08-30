# CLI and live authoring

md0 exposes both explicit subcommands for scripts and a human-oriented document launcher.

## Document launcher

Run a document path directly:

```bash
md0 report.md
```

In an interactive terminal md0 presents actions for the selected document:

```text
Edit + live preview
Open live viewer
Render standalone HTML
Inspect document
Validate
Quit
```

The launcher is a convenience layer only. Explicit commands remain available and are the stable choice for automation.

## Pretty terminal output

Human-facing commands use a small md0-owned terminal presentation layer with ANSI color and Unicode status symbols when stdout is a terminal. Output automatically falls back to plain text when color is unsuitable, including `TERM=dumb`, redirected output, and environments that set `NO_COLOR`.

The implementation uses only the Go standard library. md0 does not import Bubble Tea, Lip Gloss, Cobra, a color package, or another CLI framework.

Machine-relevant behavior remains explicit: `md0 eval` and `md0 inspect` retain plain textual output, and `md0 render` without `-o` still writes only rendered HTML to stdout.

## Live authoring

Start the built-in authoring surface with:

```bash
md0 edit report.md
```

The browser shows the source editor beside the actual md0 document preview.

Authoring follows a two-stage model:

1. Typing creates an **in-memory draft**. The draft is parsed, evaluated, and rendered by the md0 runtime without writing the source file.
2. `Cmd+S`, `Ctrl+S`, or the Save button explicitly commits the editor contents to the one document path selected on the command line.

This means malformed half-written syntax can produce a diagnostic while the last valid preview remains visible. Editing does not grant the document language filesystem authority.

## Live preview

Draft rendering uses the normal parser, evaluator, attachment bindings, document renderer, MathML renderer, and SVG plot renderer. Changes to prose, directives, mathematical notation, plots, inputs, tables, and charts therefore appear through the same rendering path used by normal md0 documents.

The existing source watcher remains active. If another editor changes the selected file on disk, the browser detects the new source revision and reloads the live document.

## Save boundary

The authoring server may write **only the file path explicitly supplied by the host operator** to `md0 edit`. Source code inside an md0 document cannot choose another path or invoke the save endpoint itself.

Editor requests are protected by the same loopback Host, Origin, and `Sec-Fetch-Site` checks as the viewer plus a separate cryptographically random editor capability token. Source and draft request bodies retain the existing 2 MiB document bound.

The editor preserves the existing file permissions when saving.

## Keyboard behavior

```text
Cmd+S / Ctrl+S   save current source
```

The source pane remains a normal text-editing control so browser/OS selection, undo, redo, and navigation behavior continue to work.

## Relationship to `md0 open`

Use `md0 open` when the Markdown source is being edited elsewhere and md0 should only provide the reactive viewer.

Use `md0 edit` when the source and preview should live together in the md0 browser surface.

Both modes are loopback-only and keep the md0/PURE document authority boundary unchanged.
