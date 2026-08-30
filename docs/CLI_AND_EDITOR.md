# CLI and live authoring

md0 exposes explicit subcommands for scripts and a persistent human-oriented terminal application for working with one document.

## Document app

Run a document path directly:

```bash
md0 report.md
```

In an interactive terminal, md0 enters a full-screen document application and stays alive while you move between workflows:

```text
Edit document
Open live viewer
Render standalone HTML
Inspect document
Validate
Quit
```

Use the Up/Down arrows or `j`/`k` to move, then press Enter. The displayed shortcut keys (`e`, `o`, `r`, `i`, `v`, and `q`) select an action immediately.

The home screen is not a one-shot launcher. It is the top level of the md0 terminal app:

- `e` enters the full-screen editor; `Escape` returns to the md0 home screen;
- `o` starts or reopens the live browser viewer while the terminal app remains running;
- `r` renders standalone HTML and returns home with a status message;
- `i` opens an in-app inspect view; `Escape` or Enter returns home;
- `v` validates without terminating the app when the document is invalid;
- `q` or `Escape` on the home screen exits md0.

The app owns one alternate-screen/raw-input session, so moving between home, editor, diagnostics, and inspection feels like switching views in one terminal application rather than launching a sequence of commands. Explicit commands remain available and are the stable choice for automation or direct entry into one workflow.

## Pretty terminal output

Human-facing commands use a small md0-owned terminal presentation layer with ANSI color and Unicode status symbols when stdout is a terminal. Output automatically falls back to plain text when color is unsuitable, including `TERM=dumb`, redirected output, and environments that set `NO_COLOR`.

The default terminal palette uses warm ivory for `MD`, coral for `0`, muted sand for secondary accents, and green for successful operations. Every role can be customized without allowing raw terminal escape sequences:

```bash
MD0_COLOR_MD='#f5f2dc' \
MD0_COLOR_ZERO=71 \
MD0_COLOR_ACCENT=coral \
md0 report.md
```

The supported variables are `MD0_COLOR_MD`, `MD0_COLOR_ZERO`, `MD0_COLOR_ACCENT`, `MD0_COLOR_SECONDARY`, `MD0_COLOR_SUCCESS`, and `MD0_COLOR_ERROR`. A value may be a named terminal color, a 256-color index from `0` through `255`, or an exact `#RRGGBB` color. Invalid values safely fall back to the default palette. `NO_COLOR` still disables all color.

The implementation uses only the Go standard library. md0 does not import Bubble Tea, Lip Gloss, Cobra, a color package, or another CLI framework.

Machine-relevant behavior remains explicit: `md0 eval` and `md0 inspect` retain plain textual output, and `md0 render` without `-o` still writes only rendered HTML to stdout.

## Terminal authoring

From the document app, press `e` to enter the editor and `Escape` to return home. `md0 edit` remains a direct shortcut when only the editor is wanted:

```bash
md0 edit report.md
```

The editor stays entirely in the terminal, opens the selected source immediately, and does not start an HTTP server or browser. It has a full-screen buffer with line numbers, cursor movement, syntax colors, and md0-aware completion. It remains useful before a document parses successfully, so a broken draft can still be repaired.

Key behavior:

- `Ctrl+S` saves and keeps the file's existing permissions;
- `Escape` returns to the md0 home screen; with unsaved edits the first press warns and the second discards;
- `Ctrl+Q` remains an alternate quit/back binding where the host terminal forwards it;
- arrows, Home/End, Page Up/Down, Backspace/Delete, Enter, and Tab edit the buffer;
- typing relevant md0 syntax opens a compact cursor-local completion popup without shrinking the document viewport;
- the selected completion can show a dim inline ghost continuation;
- `Ctrl+Space` explicitly opens broader suggestions for directives, input/data types, symbols, expressions, table/chart fields, and plot functions;
- `Up`/`Down` selects a suggestion and `Enter`/`Tab` inserts it;
- `Escape` closes an open completion popup before it is treated as navigation back to the app;
- block completions can scaffold `@when`, `@assert`, `@table`, `@chart`, and fenced `plot` structures.

Saves include the revision opened by the editor. If another process changes the file first, the save is rejected so a newer edit cannot be overwritten.

## Browser authoring

The one live viewer is available either from `o` inside `md0 FILE` or directly with:

```bash
md0 open report.md
```

Open the `Aa` Settings button and choose **Edit source** when you want a browser source pane. The pane is a toggle inside the viewer, not a second browser mode; closing it returns to the same viewer. Browser drafts are parsed and previewed in memory, and Save is explicit.

The browser source pane keeps native textarea input, selection, undo, redo, IME, and accessibility behavior while adding:

- synchronized line numbers and active-line highlighting;
- Markdown, md0 directive, expression, interpolation, table/chart, and `plot` syntax colors;
- directive and block snippets after typing `@`;
- in-scope document symbols and builtin function completion inside expressions;
- table, chart, and plot field suggestions based on the current block;
- keyboard completion and two-space indentation.

No editor package, JavaScript library, CDN, or network service is loaded.

Authoring follows a two-stage model:

1. Typing creates an **in-memory draft**. The draft is parsed, evaluated, and rendered by the md0 runtime without writing the source file.
2. `Cmd+S`, `Ctrl+S`, or the Save button explicitly commits the editor contents to the one document path selected on the command line.

This means malformed half-written syntax can produce a diagnostic while the last valid preview remains visible. Editing does not grant the document language filesystem authority.

## Live preview

Draft rendering uses the normal parser, evaluator, attachment bindings, document renderer, MathML renderer, and SVG plot renderer. Changes to prose, directives, mathematical notation, plots, inputs, tables, and charts therefore appear through the same rendering path used by normal md0 documents.

The existing source watcher remains active. If another editor changes the selected file on disk, the browser detects the new source revision and reloads the live document.

If the browser has unsaved edits, an external change is reported instead of silently discarding the draft. Saves include the source revision that the editor opened; a stale save is rejected rather than overwriting a newer disk version.

## Save boundary

The authoring server may write **only the file path explicitly supplied by the host operator** to `md0 open`. Source code inside an md0 document cannot choose another path or invoke the save endpoint itself.

Editor requests are protected by the same loopback Host, Origin, and `Sec-Fetch-Site` checks as the viewer plus a separate cryptographically random editor capability token. Source and draft request bodies retain the existing 2 MiB document bound.

The editor preserves the existing file permissions when saving.

## Browser pane keyboard behavior

```text
Cmd+S / Ctrl+S   save current source
Ctrl+Space       show md0 syntax suggestions
Up / Down        move through suggestions
Enter / Tab      insert the selected suggestion
Escape           close suggestions
Tab / Shift+Tab  indent / outdent source
```

The source pane remains a normal text-editing control so browser/OS selection, undo, redo, and navigation behavior continue to work.

## Relationship between terminal and web editing

Use bare `md0 FILE` as the primary interactive experience. It keeps the terminal app alive while you edit, inspect, validate, render, and start the viewer. Use `md0 edit` or `md0 open` when you specifically want to enter one workflow directly. The browser viewer's Settings → Edit source pane remains optional and does not create a second browser mode.

Both terminal and browser authoring keep the md0/PURE document authority boundary unchanged.
