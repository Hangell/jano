# Review of the routing expansion

Review date: 2026-10-08. Scope: routing snapshots, lookup/precedence, parameter
isolation, route groups, Context/error handling, optional middleware, repository
integration, compatibility and HTTP lifecycle behavior.

## Findings corrected before publication

| Finding | Impact | Correction and regression evidence |
| --- | --- | --- |
| A successful connection hijack was not tracked by Context or Recovery. | A returned error or panic could attempt a new HTTP response after ownership transferred to the handler. | Both wrappers delegate Hijack through ResponseController and mark successful takeover as committed. Tests cover late errors, panics and unsupported writers. |
| HTTP 101 was treated like provisional informational headers. | Error handling could try to overwrite a completed protocol upgrade. | Treat Switching Protocols as final in both wrappers. Regression tests verify that no error body is appended after 101. |
| The service example returned from main when ListenAndServe stopped, before Shutdown necessarily finished. | The process could interrupt active requests during graceful shutdown. | Wait on shutdown completion; force-close remaining connections when the five-second deadline expires. A test with an active HTTP request verifies completion before shutdown returns. |

Connection tests also verify that ResponseController write deadlines pass through
both wrappers. The hijack contract follows the [Go HTTP API](https://pkg.go.dev/net/http#Hijacker),
and the example shutdown follows the [Go server lifecycle contract](https://pkg.go.dev/net/http#Server.Shutdown).

## Validation

- `make check`: formatting, vet, race-enabled tests and build passed on Go 1.27.1.
- `GOTOOLCHAIN=go1.22.5 make check`: the same checks passed on the minimum version.
- Root-package statement coverage: 99.4%, measured with `go test -coverprofile=coverage.out .`.
- Route matching fuzz tests passed with `-fuzz=FuzzMatchRoute -fuzztime=10s -parallel=2`.
- Existing legacy API and CRUD tests continue to pass.
- New tests cover deterministic precedence, backtracking, catch-alls, validation,
  all group method helpers, middleware scopes, first-request compilation,
  concurrent reconfiguration, JSON limits, safe error responses and cancellation.
- The SQL adapter is tested through a local database/sql test driver; there is no
  live PostgreSQL, sqlx or GORM integration test in this increment.
- README examples were compiled and local documentation links checked.
- Benchmark method and raw before/after results are recorded in [benchmarks.md](benchmarks.md).

## Compatibility and boundaries

No unresolved blocker was found within the reviewed scope. This is a self-review
with automated tests, not an independent security audit or proof of framework parity.

Existing ordinary calls to New, Get/Post and the other method helpers, Use,
NotFound and Router remain valid. New accepts variadic options, which changes its
function type for consumers storing it as `func() *Jano`; those consumers need a
zero-argument wrapper. Malformed/repeated parameters and structurally duplicate
patterns now fail registration, static precedence is deterministic, and a final
`{name...}` is a catch-all. These migration changes are recorded in the commit's
breaking-change footer and [architecture.md](architecture.md).

405 remains opt-in; HEAD/OPTIONS are explicit. Global middleware continues to
bypass 404/405 unless applied around the entire app. Group middleware is captured
at registration. Context deadlines require cooperating downstream operations.
Optional legacy writer interfaces such as Flusher are not all exposed by the
wrappers; use ResponseController or an integration-specific adapter. Application
code owns database pools and hijacked connections.
