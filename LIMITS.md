# Runtime Limits

md0/PURE intentionally bounds several pieces of document execution so malformed or adversarial input cannot grow work without limit.

| Limit | Current v1 bound | Enforced by |
|---|---:|---|
| Document size | 2 MiB | `ParseFile` / `ParseString` |
| File-backed line size | 256 KiB | `bufio.Scanner` maximum buffer |
| Expression size | 512 lexer tokens | bounded expression preflight |
| Nested block depth | 64 levels | recursive block parser |
| Bar-chart values | 128 | chart validator |
| Table columns | 64 | table validator |
| Table rows | 1,000 | table validator |

## What these limits mean

The 2 MiB document bound applies before parsing. `ParseFile` also reads through a scanner capped at 256 KiB for an individual line, so extremely long single-line documents can fail before reaching the overall document-size ceiling.

Every directive expression is lexed through a bounded preflight before the recursive-descent expression parser runs. More than 512 tokens is rejected before expression parsing.

Nested md0 block structure is capped at 64 levels. This includes recursive block parsing such as nested conditional regions.

Charts and tables have separate output-shape limits because they can multiply rendered DOM/SVG work even when their source expressions are small.

## Scale tested in CI

The performance harness currently exercises synthetic **5,000-calculation** dependency chains and **5,000-calculation** fan-out documents on every main-branch CI run. This is a tested scale, not a language maximum. See `PERFORMANCE.md` for the measured snapshot and methodology.

## PURE authority boundary

These resource bounds are separate from md0/PURE's authority restrictions. A document still has no syntax or evaluator primitive for filesystem access, arbitrary network access, shell/process spawning, environment variables, package imports, native code, or dynamic evaluation.
