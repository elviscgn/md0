# Security

md0 is designed around one rule: **interactivity without authority**.

An md0/PURE document may calculate, react to typed inputs, conditionally render, assert, and produce tables/charts. It has no document-language primitive for shell execution, process spawning, arbitrary filesystem access, network sockets, environment variables, package imports, native code, or dynamic evaluation.

## Threat model

The runtime treats the `.md` document and live input values as untrusted data. The goal is to prevent those values from becoming ambient host authority or executable browser markup.

The host operator is trusted to choose which file to open and which address `md0 open` binds to. The CLI defaults to `127.0.0.1:8080`; deliberately supplying a non-loopback address exposes the local runtime beyond the machine and should be treated as an explicit operator decision.

## Document boundary

md0/PURE keeps dangerous capabilities absent from the document language rather than granting and attempting to filter them later.

Resource work is bounded independently of authority:

- document size: 2 MiB
- file-backed line size: 256 KiB
- expression preflight: 512 lexer tokens
- nested blocks: 64 levels
- bar-chart values: 128
- table columns: 64
- table rows: 1,000

See `LIMITS.md` for the current bounds and `PERFORMANCE.md` for the measured scale harness.

## Browser rendering boundary

Document prose, interpolated values, table cells, chart labels, titles, assertion content, and code spans are emitted through renderer-controlled markup and HTML escaping. Raw document HTML is not passed through as executable HTML.

The interactive runtime additionally sends a restrictive Content Security Policy:

- `default-src 'none'`
- inline runtime scripts and styles are allowed only by exact SHA-256 hashes
- no `unsafe-inline` or `unsafe-eval`
- `connect-src 'self'`
- `base-uri 'none'`
- `form-action 'none'`
- `frame-ancestors 'none'`

Responses also set `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and `Cross-Origin-Resource-Policy: same-origin`.

## Local HTTP boundary

`POST /render` accepts only `application/json`, caps the request body at 1 MiB, and requires exactly one JSON object. Unknown input names and invalid typed values are rejected by the reactive evaluator without committing partial state.

The local HTTP server sets read-header, read, write, idle, and maximum-header limits so slow or oversized requests cannot remain unbounded by default. The runtime does not enable CORS.

## Adversarial regression corpus

`internal/md0/security_test.go` exercises security-sensitive behavior including:

- HTML/script-shaped prose and values must remain escaped
- CSP/security headers and route boundaries
- wrong content types, ambiguous JSON, null and oversized request bodies
- bad-request followed by valid-request state recovery
- document-size rejection
- expression-token exhaustion
- excessive block nesting
- chart/table shape limits

CI runs this corpus explicitly in addition to the full unit suite, race detector, parser/evaluator fuzzing, 5,000-node scale benchmarks, zero-third-party-module proof, and cross-platform tests.

## Reporting

For a security issue, please avoid publishing exploit details before a fix is available. Use the repository owner's private contact or GitHub's private vulnerability reporting feature when enabled.
