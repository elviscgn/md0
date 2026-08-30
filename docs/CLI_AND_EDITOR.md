# CLI and live authoring

md0 exposes both explicit subcommands for scripts and a human-oriented document launcher.

## Document launcher

Run a document path directly:

```bash
md0 report.md
```

In an interactive terminal md0 presents actions for the selected document:

```text
Edit in terminal
Open live viewer
Render standalone HTML
Inspect document
Validate
Quit
```

Use the Up/Down arrows or `j`/`k` to move, then press Enter. The displayed
shortcut keys (`e`, `o`, `r`, `i`, `v`, and `q`) select an action immediately.
If the terminal cannot enter interactive input mode, md0 falls back to typing a
shortcut followed by Enter. The interactive launcher clears the terminal before
drawing and clears its menu again before the selected command starts.

The launcher is a convenience layer only. Explicit commands remain available and are the stable choice for automation.

## Pretty terminal output

Human-facing commands use a small md0-owned terminal presentation layer with ANSI color and Unicode status symbols when stdout is a terminal. Output automatically falls back to plain text when color is unsuitable, including `TERM=dumb`, redirected output, and environments that set `NO_COLOR`.

The default launcher palette uses warm ivory for `MD`, coral for `0`, muted
sand for secondary accents, and green for successful operations. Every role can
be customized without allowing raw terminal escape sequences:

```bash
MD0_COLOR_MD='#f5f2dc' \
MD0_COLOR_ZERO=71 \
MD0_COLOR_ACCENT=coral \
md0 report.md
```

The supported variables are `MD0_COLOR_MD`, `MD0_COLOR_ZERO`,
`MD0_COLOR_ACCENT`, `MD0_COLOR_SECONDARY`, `MD0_COLOR_SUCCESS`, and
`MD0_COLOR_ERROR`. A value may be a named terminal color, a 256-color index
from `0` through `255`, or an exact `#RRGGBB` color. Invalid values safely fall
back to the default palette. `NO_COLOR` still disables all color.

The implementation uses only the Go standard library. md0 does not import Bubble Tea, Lip Gloss, Cobra, a color package, or another CLI framework.

Machine-relevant behavior remains explicit: `md0 eval` and `md0 inspect` retain plain textual output, and `md0 render` without `-o` still writes only rendered HTML to stdout.

## Live authoring

`md0 edit` is the terminal editor. It stays in the terminal, opens the selected
source immediately, and does not start an HTTP server or a browser:

```bash
md0 edit report.md
```

The editor has a Vim-like full-screen buffer with line numbers, cursor movement,
syntax colors, and md0-aware completion. It is intentionally useful before a
document parses successfully, so a broken draft can still be repaired:

- `Ctrl+S` saves and keeps the file's existing permissions;
- `Ctrl+Q` quits, asking for a second press when there are unsaved changes;
- arrows, Home/End, Page Up/Down, Backspace/Delete, Enter, and Tab edit the buffer;
- `Ctrl+Space` opens suggestions for directives, input/data types, symbols,
  expressions, table/chart fields, and plot functions;
- `Up`/`Down` selects a suggestion and `Enter`/`Tab` inserts it;
- `Escape` closes the completion list.

Saves include the revision opened by the editor. If another process changes the
file first, the save is rejected so a newer edit cannot be overwritten.

Web editing is available from the one live viewer. Start it with:

```bash
md0 open report.md
```

Open the `Aa` Settings button and choose **Edit source** when you want a browser
source pane. The pane is a toggle inside the viewer, not a second browser mode;
closing it returns to the same viewer. Browser drafts are parsed and previewed in
memory, and Save is explicit.

The browser source pane keeps native textarea input, selection, undo, redo, IME,
and accessibility behavior while adding:

- synchronized line numbers and active-line highlighting;
- Markdown, md0 directive, expression, interpolation, table/chart, and `plot`
  syntax colors;
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

If the browser has unsaved edits, an external change is reported instead of
silently discarding the draft. Saves include the source revision that the
editor opened; a stale save is rejected rather than overwriting a newer disk
version.

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

Use `md0 edit` for terminal-first authoring. Use `md0 open` for the reactive
viewer, and enable its Settings → Edit source pane only when browser editing is
useful. `md0 open` accepts `--no-browser` when the URL should be printed without
launching a browser.

Both paths keep the md0/PURE document authority boundary unchanged.
