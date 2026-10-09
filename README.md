# Jano

[![CI](https://github.com/Hangell/jano/actions/workflows/ci.yml/badge.svg)](https://github.com/Hangell/jano/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Hangell/jano.svg)](https://pkg.go.dev/github.com/Hangell/jano)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue.svg)](LICENSE)

Jano is a Go HTTP routing library with an Express-inspired API, built on
`net/http` with no external dependencies. It provides deterministic routing,
route groups, middleware, JSON helpers and error-returning handlers while keeping
standard Go HTTP handlers compatible.

## Installation

Requires Go **1.22.5 or newer**, as declared in [go.mod](go.mod).

```sh
go get github.com/Hangell/jano
```

The module path is case-sensitive: use `github.com/Hangell/jano`.

## Quick start

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/Hangell/jano"
)

func main() {
    app := jano.New()
    app.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            log.Printf("%s %s", r.Method, r.URL.Path)
            next.ServeHTTP(w, r)
        })
    })
    app.Get("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Welcome to Jano!")
    })
    app.Get("/people/{id}", func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        fmt.Fprintf(w, "Person: %s\n", id)
    })
    app.NotFound(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "Page not found", http.StatusNotFound)
    })
    log.Fatal(http.ListenAndServe(":8080", app.Router()))
}
```

## Context handlers and route groups

Use `HandleContext` for request helpers and centralized error handling. Existing
`Get`, `Post` and other method helpers continue to accept `http.HandlerFunc`.

```go
app := jano.New(jano.WithMethodNotAllowed())
api := app.Group("/api").Group("/v1")
api.HandleContext(http.MethodPost, "/people", func(c *jano.Context) error {
    var input struct {
        Name string `json:"name"`
    }
    if err := c.BindJSON(&input); err != nil {
        return err
    }
    if input.Name == "" {
        return jano.NewHTTPError(http.StatusBadRequest, "Name is required")
    }
    return c.JSON(http.StatusCreated, input)
})
```

`BindJSON` accepts one value, rejects unknown fields and limits the body to 1 MiB.
Use `BindJSONLimit` to choose another limit. Domain validation remains explicit.
Unexpected errors produce a generic JSON 500 response; `HTTPError` exposes only
its client-safe message. Customize logging or error responses with `SetErrorHandler`.
Response helpers return errors and prevent writing a second response.

## API

| API | Purpose |
| --- | --- |
| `New(options ...Option)` | Create a router; `WithMethodNotAllowed()` opts into 405 responses. |
| `Get`, `Post`, `Put`, `Delete`, `Patch`, `Options`, `Head` | Register a standard `http.HandlerFunc`. |
| `Handle(method, path, http.Handler, ...Middleware)` | Register any HTTP handler, including route middleware. |
| `HandleContext(method, path, HandlerFunc, ...Middleware)` | Register `func(*Context) error`. |
| `Register`, `RegisterContext` | Return configuration errors instead of panicking. |
| `Group(prefix, ...Middleware)` | Create nested groups with inherited middleware. |
| `Use(Middleware)` | Add global middleware in registration order. |
| `NotFound(http.HandlerFunc)` | Replace the fallback; set its HTTP status explicitly. |
| `SetErrorHandler(ErrorHandler)` | Customize the error policy for context handlers. |
| `Router()` / `ServeHTTP` | Serve Jano directly as an `http.Handler`. |
| `Param(request, name)` | Read string parameters through `Request.PathValue`. |
| `ParamInt`, `ParamInt64` | Convert decimal parameters with range checks and HTTP 400 errors. |
| `Context.Param`, `ParamInt`, `ParamInt64`, `Query`, `Context` | Access string/numeric parameters, query and request context. |
| `Context.JSON`, `Text`, `NoContent` | Write a response and return write/encoding errors. |
| `Context.BindJSON`, `BindJSONLimit` | Decode bounded JSON without automatically writing a response. |

### Routing behavior

- Parameters occupy an entire segment: `/people/{id}`. Prefer `jano.Param(r, "id")`
  or `r.PathValue("id")`; legacy `r.Context().Value("id")` also works.
- A final `{path...}` captures the remaining path: `/files/{path...}` matches
  `/files/a/b` and `/files/`, but not `/files` without the separator.
- Matching uses the decoded `URL.Path`, is case-sensitive and preserves trailing
  slashes. Query strings do not participate in matching.
- Static segments take precedence over parameters, which take precedence over
  catch-alls. If a more specific branch fails, lookup tries the next matching
  branch. Registration order does not affect precedence.
- Registering the same method and path again replaces the previous handler.
  Structurally identical patterns with different parameter names for the same
  method are rejected, as are empty/repeated parameter names and malformed paths.
  `Handle` and method helpers panic on invalid configuration; `Register` returns
  an error. Regular-expression constraints are not supported.
- `HEAD` and `OPTIONS` must be registered explicitly. Unregistered methods use
  the fallback by default; `WithMethodNotAllowed()` enables 405 with a sorted
  `Allow` header when another method matches the requested path.
- Middleware order is global → parent group → child group → route → handler.
  Group middleware is captured at registration; group `Use` affects future routes.
  Global `Use` applies to all registered routes after recompilation.
- Global middleware runs only for matched routes. To apply recovery, tracing or
  authentication to all requests, including 404/405, wrap the complete app as a
  standard HTTP handler.
- Configuration changes are synchronized. Each request uses an immutable snapshot;
  a request already in progress may finish with the previous configuration.
  Prefer configuring before startup to avoid recompilation. Middleware factories
  must be side-effect free; handlers and services must protect shared mutable state.
- Context response tracking supports streaming/connection operations through
  `http.ResponseController`. Hijacking is tracked; unsupported writers return `http.ErrNotSupported`.
  Optional legacy writer interfaces are not all exposed
  by wrappers; use standard handlers when an integration requires those assertions.

## Numeric IDs and the native Go router

Go 1.22 added method patterns and parameters to `http.ServeMux`; read their values
with `Request.PathValue`. Values remain strings, so numeric conversion and domain
validation are still required. See the [official Go routing guide](https://go.dev/blog/routing-enhancements).

Jano's parameter helpers work with both routers:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
    id, err := jano.ParamInt64(r, "id")
    if err != nil || id <= 0 {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    fmt.Fprintf(w, "User: %d", id)
})
```

In a context handler, use `c.ParamInt64("id")` or `c.ParamInt("id")` and return
conversion errors to the error policy. Missing, malformed or overflowing values
return zero and an `HTTPError` with status 400. Helpers do not write a response.
`ParamInt` uses the platform's `int` range; `ParamInt64` has a fixed signed 64-bit
range. Signed numbers, zero and leading zeros are allowed by conversion; check
positive-only ID rules explicitly. UUIDs and other identifiers remain strings via
`Param` or `PathValue` and require their own domain validation.

Using the helpers with ServeMux does not give both routers identical semantics.
ServeMux controls its own HEAD handling, redirects and escaped-segment matching;
Jano's routing behavior remains as documented above.

## Optional middleware

The `github.com/Hangell/jano/middleware` package provides:

| Middleware | Behavior |
| --- | --- |
| `Recovery(report)` | Recover panics with generic 500 responses; report them through an optional callback. |
| `RequestID` | Generate a request ID in the response header and request context. |
| `BodyLimit(bytes)` | Bound request bodies; handlers must handle streamed read errors. |
| `ContextTimeout(duration)` | Set a deadline for context-aware operations; it does not force-stop handlers. |

```go
app.Use(middleware.Recovery(nil))
app.Use(middleware.RequestID)
app.Use(middleware.ContextTimeout(2 * time.Second))
```

Middleware remains ordinary `func(http.Handler) http.Handler`, so third-party
middleware and handlers can be composed with Jano. See
[architecture and integration](docs/architecture.md) for contracts and examples.

## Database and service integration

Inject application-owned repositories into handlers. Jano does not open database
connections or impose an ORM. Pass `c.Context()` to repository operations so
request cancellation and deadlines reach context-aware drivers.

The [service example](examples/service) includes a repository interface, an
in-memory implementation, a PostgreSQL `database/sql` adapter, error mapping,
request IDs, deadlines and graceful HTTP shutdown:

```sh
go run ./examples/service
curl http://localhost:9001/api/v1/users/1
```

It runs with in-memory data. To use the SQL adapter, register a PostgreSQL driver,
open a database in your application, create `users(id, name)`, and inject
`SQLUsers{DB: db}`. Jano has no runtime dependency on that driver. The same
repository contract can be implemented using sqlx, GORM, Redis or another service.
SQL tests use a local test driver; they do not connect to a real database.

## Runnable CRUD example

```sh
go run ./examples/api
```

The example listens on port `9000`, or the port specified by `PORT`. It stores
people in memory, so restarting it clears the data. Example requests are in
[examples/api/requests.http](examples/api/requests.http).

```sh
curl -X POST http://localhost:9000/people \
  -H 'Content-Type: application/json' \
  -d '{"name":"Jane","age":30,"student":false}'
curl http://localhost:9000/people/1
```

## Project layout

```text
jano/
├── .github/workflows/ci.yml
├── go.mod
├── jano.go                 # Public router
├── jano_test.go
├── options.go              # Functional options
├── errors.go
├── context.go
├── params.go               # String and numeric path helpers
├── group.go
├── router.go               # Routing implementation
├── internal/
│   └── routepattern/       # Private route validation
├── middleware/             # Public optional middleware
├── examples/               # CRUD and repository integration
└── docs/                   # Architecture, benchmarks and review
```

The public package remains importable as `github.com/Hangell/jano`. See the
[layout rationale](docs/architecture.md#module-layout) and the
[official Go guide](https://go.dev/doc/modules/layout).

## Development

```sh
make check       # formatting, vet, race-enabled tests and build
make coverage    # coverage.out and coverage.html
make bench       # benchmarks with allocation measurements
make fuzz        # fuzz route matching for 10 seconds
```

`make` is optional: equivalent Go commands are in
[CONTRIBUTING.md](CONTRIBUTING.md). See the [benchmark methodology](docs/benchmarks.md).
Benchmarks depend on Go version, hardware and workload; they do not establish
feature or performance parity with other frameworks.

## Contributing

Bug reports, tests, documentation and focused improvements are welcome.
Read the [contribution guide](CONTRIBUTING.md),
[code of conduct](CODE_OF_CONDUCT.md) and [security policy](SECURITY.md).
Issues and pull requests may be written in English or Portuguese.

## Name and author

Jano is named after Janus, the Roman god associated with beginnings and transitions.
Created by [Rodrigo Rangel (Hangell)](https://github.com/Hangell).

## License and support

Distributed under the [BSD-3-Clause license](LICENSE).
If you would like to support development, the existing cryptocurrency donation
address is `0xEd4d1be72F807Faa358C966a8eF63367c200130F`.
