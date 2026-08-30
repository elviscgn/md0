# STDLIB.md

md0 intentionally ships with **zero third-party runtime dependencies**.

| What a typical implementation might import | md0 uses instead |
|---|---|
| CLI framework | `flag`, `os`, and a small explicit subcommand dispatcher |
| Markdown parser | Hand-written line-oriented Markdown subset parser |
| Expression language / evaluator | Hand-written lexer, recursive-descent parser, and typed evaluator |
| Template/interpolation engine | A bounded `{{ expression }}` interpolator implemented with the md0 evaluator |
| HTTP router/framework | `net/http` `ServeMux` |
| JSON package | `encoding/json` |
| Session/capability helper | `crypto/rand` plus a small bounded in-memory store |
| CSP hashing | `crypto/sha256` + `encoding/base64` |
| URL / host validation | `net`, `net/url`, and `mime` |
| Charting library | Inline SVG generated directly by md0 |
| HTML escaping/sanitizing helper | `html.EscapeString` plus renderer-controlled markup |
| Filesystem abstraction | `os`, `bufio`, `io` |
| Numeric helper package | `math`, `strconv` |
| Collection helpers | Built-in slices/maps plus `sort` |
| Test assertion package | Go's built-in `testing` package |
| Duration parser | `time.ParseDuration` |

## Security boundary

The Go runtime itself can read the document chosen by the user and, for `md0 open`, bind a loopback HTTP server. **The md0/PURE document language cannot request filesystem access, network access, process spawning, environment variables, package imports, native code, or dynamic evaluation.** Those operations have no syntax and no evaluator primitive.

The loopback viewer's capability tokens, CSP hashes, host/origin checks, request limits, and session store are also implemented only with Go's standard library. See `SECURITY.md` for the threat model.

## Runtime dependency proof

`go.mod` contains no `require` directive. The strongest quick check is the module build list:

```sh
go list -m all
```

Expected output:

```text
github.com/elviscgn/md0
```

To inspect the module identity of **every package in md0's complete dependency closure**, run:

```sh
go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}' ./... | sort -u
```

Expected output: **nothing**. Standard-library packages have no external module attached to md0, while any third-party module package would appear here and fail CI.

### Why can `go list -deps` show `vendor/golang.org/x/...`?

A raw package listing may include names such as:

```text
vendor/golang.org/x/crypto/...
vendor/golang.org/x/net/...
vendor/golang.org/x/text/...
```

Those paths are implementation packages bundled inside the Go distribution's own `GOROOT`. They are used internally by the standard library; they are **not vendored by md0**, are not present in md0's module build list, and do not create a third-party module dependency for this repository.

That distinction is why md0's CI verifies module ownership rather than treating the text `vendor/` in an import path as evidence of an application dependency.

## CI enforcement

The `Prove zero third-party dependencies` CI step rejects the build if any of these become true:

- `go list -m all` contains a non-main module;
- any package in `go list -deps ./...` belongs to a non-main module;
- `go.mod` gains a `require` directive; or
- `go mod tidy` changes `go.mod` or creates/changes `go.sum`.

The workflow actions themselves are pinned to immutable commit SHAs. The runtime-dependency proof is executable and checked on every CI run, not just a documentation claim.
