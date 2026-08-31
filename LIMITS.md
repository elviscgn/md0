# Runtime Limits

md0/PURE intentionally bounds document execution and local-runtime I/O so malformed or adversarial input cannot grow work without limit.

| Limit | Current v0.1 bound | Enforced by |
|---|---:|---|
| Document size | 2 MiB | `ParseFile` / `ParseString` |
| Document encoding | valid UTF-8 | parser boundary |
| File-backed line size | 256 KiB | `bufio.Scanner` maximum buffer |
| Expression size | 512 lexer tokens | bounded expression preflight |
| Expression nesting | 128 levels | expression parser |
| Nested block depth | 64 levels | recursive block parser |
| Live string/text input | 16 KiB | input validator |
| Values JSON / snapshot input | 1 MiB | values-file loader |
| Data attachments | 16 files, 2 MiB each, 8 MiB combined | explicit attachment loader |
| JSON attachment shape | 32 levels, 100,000 values | recursive data converter |
| CSV attachment shape | 64 columns, 1,000 rows | CSV data converter |
| Computed string value | 1 MiB | expression evaluator |
| Interpolated Markdown region | 4 MiB | interpolation transform |
| Rendered document / patch response | 16 MiB | bounded output wrappers |
| Bar-chart values | 128 | chart validator |
| Function-plot curves | 4 | semantic plot renderer |
| Function-plot samples | 1,024 per curve | semantic plot renderer |
| Function-plot expression text | 16 KiB per expression | bounded plot parser |
| Function-plot expression AST | 512 nodes, 128 levels | allowlisted plot walker |
| Table columns | 64 | table validator |
| Table rows | 1,000 | table validator |
| `POST /render` body | 1 MiB | `http.MaxBytesReader` |
| Live browser sessions | 32 | bounded runtime session store |
| `md0 open` bind scope | loopback only | listen-address validator |

## What these limits mean

The 2 MiB document bound applies before parsing, and documents must be valid UTF-8. `ParseFile` also reads through a scanner capped at 256 KiB for an individual line.

Every directive expression is lexed through a bounded preflight before the recursive-descent expression parser runs. More than 512 tokens is rejected before expression parsing, and recursive expression nesting is capped at 128 levels. Nested md0 block structure is capped at 64 levels.

Live string inputs are capped at 16 KiB. String-producing expressions cannot create a value larger than 1 MiB, interpolation is independently bounded at 4 MiB per Markdown transform, and user-visible render paths cap a complete document or patch response at 16 MiB. These are separate guards against evaluator and output amplification.

Host-provided values files are capped at 1 MiB. Explicit JSON and CSV attachments are capped individually and in aggregate; JSON conversion limits nesting and value count, while CSV uses the same table-oriented column and row ceilings as rendered tables. A document can name an attachment but cannot supply, construct, or discover its path.

Charts, function plots, and tables have output-shape limits because they can multiply rendered DOM/SVG work even when their source expressions are small. A semantic `plot` fence may render at most four curves, and each curve is sampled at 32–1,024 points. `samples` is a bounded integer literal after optional interpolation; bare document identifiers are not resolved in that field. Each plot/range expression is limited to 16 KiB, 512 AST nodes, and 128 nesting levels. Plot expressions and reactive range bounds are numeric-only and evaluated by an allowlisted AST walker rather than arbitrary Go execution.

`md0 open` is deliberately a local viewer, not an internet application server. It refuses non-loopback listen addresses, caps each `/render` request at 1 MiB, and keeps at most 32 isolated browser sessions before evicting the oldest.

## Scale tested in CI

The performance harness exercises synthetic **5,000-calculation** dependency chains and **5,000-calculation** fan-out documents on main-branch CI. This is a tested scale, not a language maximum. See `PERFORMANCE.md` for the measured snapshot and methodology.

## PURE authority boundary

Resource bounds are separate from md0/PURE's authority restrictions. A document has no syntax or evaluator primitive for arbitrary filesystem access, arbitrary network access, shell/process spawning, environment variables, package imports, native code, or dynamic evaluation.
