# Changelog

## md0 v0.1.0 — 2026-08-31

First public release of md0: a zero-dependency runtime for reactive Markdown
that stays a document.

### Highlights

- **Reactive Markdown:** typed `@input` and `@data`, calculations, conditions,
  assertions, tables, charts, interpolation, dependency graphs, and targeted
  reactive updates.
- **Math and plots:** bounded LaTeX-like notation rendered as native MathML;
  bounded function-plot fences rendered as native SVG with multiple curves and
  standard-library math functions.
- **Terminal authoring:** a persistent full-screen app launched with `md0 FILE`,
  with syntax highlighting, cursor-local autocomplete, ghost completions,
  snippets, undo/redo, find, debounced autosave, and live browser updates.
- **Browser and static output:** reactive loopback viewer, source watching,
  settings, explicit browser saves, standalone HTML rendering, values files,
  snapshots, and CSV/JSON attachments.
- **Safety and portability:** md0/PURE authority boundaries, loopback-only
  serving, explicit resource limits, compiler-style diagnostics, fuzz/race/
  adversarial security coverage, reproducible builds, and zero third-party Go
  dependencies.

The release includes checksum-verified archives for macOS (Intel and Apple
Silicon), Linux (amd64 and arm64), and Windows (amd64).
