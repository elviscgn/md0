# Security

md0 is designed around one rule: **interactivity without authority**.

An md0/PURE document may calculate, react to typed inputs, conditionally render, assert, and produce tables/charts, mathematical notation, and bounded function plots. It has no document-language primitive for shell execution, process spawning, arbitrary filesystem access, network sockets, environment variables, package imports, native code, or dynamic evaluation.

## Threat model

The runtime treats the `.md` document, live input values, and explicitly selected data attachments as untrusted data. The goal is to prevent those values from becoming ambient host authority or executable browser markup.

The host operator and other processes already running as the same local user are trusted. `md0 open` is a hardened **local tool**, not an internet-facing application server, and it refuses non-loopback listen addresses. `md0 edit` is a local terminal buffer and does not listen on a socket.

## Document boundary

md0/PURE keeps dangerous capabilities absent from the document language rather than granting them and attempting to filter them later. Documents must be valid UTF-8, expressions and block depth are bounded, live string inputs are capped, and chart/table/plot shapes have explicit ceilings.

The expression parser propagates lexer failures at every token advance rather than continuing from stale parser state. String-producing expressions are capped at 1 MiB per computed value, and interpolation/rendered responses have independent size ceilings. Together these prevent malformed expressions and repeated string calculations from becoming unbounded parser/evaluator work. See `LIMITS.md` for the complete current bounds and `PERFORMANCE.md` for the measured scale harness.

`@data` declarations do not contain paths and do not grant filesystem access. Only the host CLI can bind a user-selected file with `--data name=FILE`. Bindings must match declarations exactly, and file size, aggregate size, JSON depth/value count, and CSV row/column shape are bounded before evaluation.

## Math and function-plot boundary

Mathematical notation is converted from a deliberately small LaTeX-like surface into renderer-controlled native MathML. It is not a TeX engine: there is no package loading, macro execution, arbitrary HTML command, file inclusion, shell escape, or external asset fetch.

Semantic `plot` fences are converted to native SVG. Plot expressions are parsed with Go's standard-library `go/parser`, but parsed syntax is **not executed as Go**. md0 walks the resulting AST itself and accepts only numeric literals, the local variable `x`, constants `pi`/`e`, basic numeric operators, and an explicit allowlist of math functions. Selectors, methods, indexing, composite literals, strings, arbitrary identifiers, and unrecognized AST nodes fail closed.

Plot work is bounded to at most four curves and 32–1,024 samples per curve. Domain errors and non-finite values produce curve gaps or a visible plot diagnostic rather than expanding authority.

Ordinary fenced code continues to suppress `{{ ... }}` interpolation. Only semantic plot fences opt into interpolation so document parameters remain explicit dependency-graph inputs.

## Browser rendering boundary

Document prose, interpolated values, table cells, chart labels, plot labels/titles, mathematical notation, assertion content, and code spans are emitted through renderer-controlled markup and HTML escaping or bounded parsed renderers. Raw document HTML is not passed through as executable HTML.

The interactive runtime sends a restrictive Content Security Policy:

- `default-src 'none'`
- built-in runtime scripts and styles are allowed only by exact SHA-256 hashes
- no `unsafe-inline` or `unsafe-eval`
- `connect-src 'self'`
- `base-uri 'none'`
- `form-action 'none'`
- `frame-ancestors 'none'`

Responses also set `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and `Cross-Origin-Resource-Policy: same-origin`.

## Local HTTP boundary

`md0 open` only accepts loopback bind addresses such as `127.0.0.1`, `localhost`, or `::1`. Requests must also present a loopback `Host` on the expected port. If an `Origin` header is present, it must be the matching loopback origin; browser requests marked cross-site by `Sec-Fetch-Site` are rejected. `md0 edit` has no HTTP listener.

Every viewer page load receives a fresh cryptographically random 256-bit capability token and a separate `ReactiveSession`. `POST /render`, `POST /snapshot`, and `POST /markdown` must present that token in `X-MD0-Token`, so independent browser tabs do not share reactive state or exports. The session store is bounded to 32 live sessions and evicts the oldest session at capacity.

The non-mutating `GET /source-status` live-authoring probe is protected by the same loopback Host, Origin, and `Sec-Fetch-Site` boundary. It returns only the current source revision or a diagnostic. The host watches only the document path explicitly selected on the command line; the document language cannot choose another path.

`POST /render` accepts only `application/json`, caps the request body at 1 MiB, and requires exactly one JSON object. Unknown input names and invalid typed values are rejected without committing partial reactive state.

## Built-in editor write boundary

`md0 open FILE` adds host-authoring endpoints around the **single source path explicitly selected by the operator** so the viewer's Settings panel can enable source editing. The document language cannot invoke these endpoints, choose a different path, or obtain the editor capability token. `md0 edit FILE` is a separate local terminal editor and does not start an HTTP server.

The editor receives a separate cryptographically random 256-bit capability token in addition to the normal loopback Host, Origin, and `Sec-Fetch-Site` checks. Editor draft and save requests must present that token.

Typing in the source pane does not mutate the filesystem. Draft source is bounded, parsed, evaluated, and rendered in memory. An explicit Save action or `Cmd/Ctrl+S` may write only the selected source path, preserving its existing file permissions. Editor saves carry the source revision originally opened by the browser and reject stale revisions instead of overwriting an external edit. The source remains subject to the 2 MiB and UTF-8 document bounds.

The editor intentionally does not expose directory listing, arbitrary-path read/write, file creation, shell execution, environment access, or attachment rebinding. Data attachments remain those selected by the host when authoring mode was started.

The HTTP server sets read-header, read, write, idle, and maximum-header limits. The runtime does not enable CORS.

## Adversarial regression corpus

The security corpus exercises security-sensitive behavior including:

- HTML/script-shaped prose and values remain escaped
- CSP hashes and absence of `unsafe-inline` / `unsafe-eval`
- Host poisoning, hostile Origin, and cross-site request rejection
- missing and invalid runtime capability tokens
- independent browser-session state and bounded session eviction
- wrong content types, ambiguous JSON, null and oversized request bodies
- bad-request followed by valid-request state recovery
- invalid UTF-8 rejection
- direct expression-parser lexer-error propagation
- document, expression-token, expression-nesting, block-depth, string-input, computed-string, interpolation-output, chart, plot, and table limits
- unsafe/unrecognized function-plot AST forms fail closed
- undeclared, missing, duplicate, malformed, oversized, and excessively nested data attachments
- live source diagnostics, recovery, and source-status request boundaries
- capability-authenticated snapshot and updated-Markdown exports
- editor source escaping, in-memory drafts, capability-token rejection, save-path scoping, and permission preservation

CI runs this corpus explicitly in addition to the full unit suite, race detector, parser/evaluator fuzzing, 5,000-node scale benchmarks, zero-third-party-module proof, reproducible-build check, and cross-platform tests.

## Assurance boundary

These controls are designed and regression-tested for the stated md0/PURE threat model. They are **not a formal proof or third-party security audit**, and no software should be represented as impossible to exploit.

## Reporting

For a security issue, please avoid publishing exploit details before a fix is available. Use the repository owner's private contact or GitHub's private vulnerability reporting feature when enabled.
