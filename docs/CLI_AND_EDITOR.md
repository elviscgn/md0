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

Use the Up/Down arrows or `j`/`k` to move, then press Enter. The displayed shortcut keys (`e`, `o`, `r`, `i`, `v`, and `q`) select an action immediately. Press `?` for the compact in-app help screen.

The home screen is not a one-shot launcher. It is the top level of the md0 terminal app:

- `e` enters the full-screen editor; `Escape` returns to the md0 home screen;
- `o` starts or reopens the live browser viewer while the terminal app remains running;
- `r` renders standalone HTML and returns home with a status message;
- `i` opens an in-app inspect view; `Escape` or Enter returns home;
- `v` validates without terminating the app when the document is invalid;
- `?` opens the keyboard/help reference;
- `q` or `Escape` on the home screen exits md0.

The app owns one alternate-screen/raw-input session, so moving between home, editor, diagnostics, inspection, and help feels like switching views in one terminal application rather than launching a sequence of commands. Explicit commands remain available and are the stable choice for automation or direct entry into one workflow.

When the viewer is started from the app, md0 prefers `127.0.0.1:8080` and searches the bounded `8080` through `8099` range if that port is already occupied. The home screen shows the active viewer address; selecting **Open live viewer** again reopens that viewer rather than starting another one.

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

The editor stays entirely in the terminal, opens the selected source immediately, and does not start an HTTP server or browser. It has a full-screen buffer with line numbers, cursor movement, syntax colors, md0-aware completion, bounded undo/redo history, and in-document search. It remains useful before a document parses successfully, so a broken draft can still be repaired.

Terminal editing is live by default. After an edit, md0 waits for a short idle period (currently 400 ms), saves the latest buffer to the explicitly opened file, and updates its source revision. If the live browser viewer is already open, its existing source watcher observes that save and refreshes the rendered document. `Ctrl+S` remains an immediate forced save.

Key behavior:

- `Ctrl+S` saves immediately and keeps the file's existing permissions;
- `Ctrl+Z` undoes and `Ctrl+Y` redoes; undo/redo changes participate in autosave so a live viewer follows them too;
- `Ctrl+F` opens a compact find bar; Enter/Down moves to the next match, Up moves to the previous match, and `Escape` closes search;
- `Escape` closes autocomplete/search first; otherwise it saves pending changes and returns to the md0 home screen (or exits a direct `md0 edit` session);
- if an external edit creates a revision conflict, md0 refuses to overwrite it and a second `Escape` can explicitly discard the local unsaved buffer;
- arrows, Home/End, Page Up/Down, Backspace/Delete, Enter, and Tab edit the buffer;
- typing relevant md0 syntax opens a compact cursor-local completion popup without shrinking the document viewport;
- the selected completion can show a dim inline ghost continuation;
- `Ctrl+Space` explicitly opens broader suggestions for directives, input/data types, symbols, expressions, table/chart fields, named curves, and plot functions;
- `Up`/`Down` selects a suggestion and `Enter`/`Tab` inserts it;
- block completions can scaffold `@when`, `@assert`, `@table`, `@chart`, and fenced `plot` structures using the preferred `f(x) = expression` syntax. Document values are suggested directly inside plot formulas.

Undo history is bounded to 128 snapshots and a bounded memory budget rather than growing without limit. Consecutive ordinary typing is coalesced into an undo group instead of creating one history entry per character.

Every terminal save, including autosave, compares the current disk revision against the revision md0 last observed. If another process changes the file first, the save is rejected instead of overwriting newer work. The existing UTF-8, 2 MiB source limit, line-ending preservation, and file-permission preservation still apply.

## Terminal-to-browser live loop

A common workflow is:

1. Run `md0 report.md`.
2. Press `o` once to start the browser viewer.
3. Return to the terminal app and press `e`.
4. Edit the source normally.
5. Pause briefly; md0 autosaves and the browser reloads the latest valid source.

The viewer currently checks source revisions every 700 ms, while terminal autosave waits 400 ms after the last edit. In normal use the browser therefore follows a terminal edit in roughly a second or less after typing stops, depending on polling phase.

A half-written or otherwise invalid autosaved document does not destroy the last good browser render. The viewer reports the source diagnostic and keeps the previous valid page visible; once the source becomes valid again, the same watcher recovers automatically.

## Browser authoring

The one live viewer is available either from `o` inside `md0 FILE` or directly with:

```bash
md0 open report.md
```

Open the `Aa` Settings button and choose **Edit source** when you want a browser source pane. The pane is a toggle inside the viewer, not a second browser mode; closing it returns to the same viewer. Browser drafts are parsed and previewed in memory, and **browser source saving remains explicit**.

The browser source pane keeps native textarea input, selection, undo, redo, IME, and accessibility behavior while adding:

- synchronized line numbers and active-line highlighting;
- Markdown, md0 directive, expression, interpolation, table/chart, and `plot` syntax colors;
- directive and block snippets after typing `@`;
- in-scope document symbols and builtin function completion inside expressions;
- table, chart, and plot field suggestions based on the current block;
- keyboard completion and two-space indentation.

No editor package, JavaScript library, CDN, or network service is loaded.

Browser authoring follows a two-stage model:

1. Typing creates an **in-memory draft**. The draft is parsed, evaluated, and rendered by the md0 runtime without writing the source file.
2. `Cmd+S`, `Ctrl+S`, or the Save button explicitly commits the editor contents to the one document path selected on the command line.

This means malformed half-written browser syntax can produce a diagnostic while the last valid preview remains visible. Editing does not grant the document language filesystem authority.

## Live preview

Draft rendering uses the normal parser, evaluator, attachment bindings, document renderer, MathML renderer, and SVG plot renderer. Changes to prose, directives, mathematical notation, plots, inputs, tables, and charts therefore appear through the same rendering path used by normal md0 documents.

The source watcher remains active. If the terminal editor or another editor changes the selected file on disk, the browser detects the new source revision and reloads the live document.

If the browser source pane has unsaved edits, an external change is reported instead of silently discarding its draft. Browser saves include the source revision that the editor opened; a stale save is rejected rather than overwriting a newer disk version.

## Save boundary

Terminal autosave and the browser source-pane Save action are **host-side authoring capabilities around the one source path explicitly selected by the operator**. Source code inside an md0 document cannot choose another path, trigger terminal saves, or invoke the browser editor endpoint itself.

The terminal editor does not listen on a socket. It serializes bounded writes to its selected path and checks SHA-256 source revisions before every write. The browser editor requests are protected by the same loopback Host, Origin, and `Sec-Fetch-Site` checks as the viewer plus a separate cryptographically random editor capability token. Source and draft request bodies retain the existing 2 MiB document bound.

Both terminal and browser saves preserve the existing file permissions.

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

Use bare `md0 FILE` as the primary interactive experience. It keeps the terminal app alive while you edit, inspect, validate, render, and start the viewer. Terminal edits autosave into the selected file so an already-running viewer can follow them. Use `md0 edit` or `md0 open` when you specifically want to enter one workflow directly. The browser viewer's Settings → Edit source pane remains optional and keeps explicit-save semantics.

Both terminal and browser authoring keep the md0/PURE document authority boundary unchanged.
