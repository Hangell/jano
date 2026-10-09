# Jano

[![CI](https://github.com/Hangell/jano/actions/workflows/ci.yml/badge.svg)](https://github.com/Hangell/jano/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Hangell/jano.svg)](https://pkg.go.dev/github.com/Hangell/jano)
[![License: BSD-3-Clause](https://img.shields.io/badge/license-BSD--3--Clause-blue.svg)](LICENSE)

Jano is a small Go HTTP routing library with an Express-inspired API, built on
`net/http` with no external dependencies.

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
        id, _ := r.Context().Value("id").(string)
        fmt.Fprintf(w, "Person: %s\n", id)
    })
    app.NotFound(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "Page not found", http.StatusNotFound)
    })
    log.Fatal(http.ListenAndServe(":8080", app.Router()))
}
```

## API

| Method | Purpose |
| --- | --- |
| `New()` | Create a router with a default HTTP 404 handler. |
| `Get`, `Post`, `Put`, `Delete`, `Patch`, `Options`, `Head` | Register a handler for a method and path. |
| `Use(func(http.Handler) http.Handler)` | Add middleware in registration order. |
| `NotFound(http.HandlerFunc)` | Replace the fallback handler. Set its HTTP status explicitly. |
| `Router() http.Handler` | Obtain a handler for `http.Server` or `httptest`. |

### Routing behavior

- Parameters occupy an entire segment: `/people/{id}`. Read them with
  `r.Context().Value("id").(string)`; they are also available inside middleware.
- Matching is case-sensitive. Trailing slashes are significant; `/people` and
  `/people/` are different routes. Query strings do not participate in matching.
- Registering the same method and path again replaces the previous handler.
- An unregistered method returns the fallback response (404 by default).
  `HEAD` and `OPTIONS` must be registered explicitly; there is no automatic 405.
- Middleware runs only for matched routes. The first registered middleware is
  the outermost wrapper; unmatched requests go directly to `NotFound`.
- Wildcards and regular-expression constraints are not supported. For example,
  `{id:[0-9]+}` is treated as a literal parameter name, without validation.
- Avoid overlapping patterns such as `/people/new` and `/people/{id}` for the
  same method. Route lookup uses a map, so precedence is unspecified.
- Configure routes, middleware and the fallback **before** serving requests.
  Concurrent configuration is unsupported. Handlers and middleware must protect
  any shared mutable state used by concurrent requests.

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

## Development

```sh
make check       # formatting, vet, race-enabled tests and build
make coverage    # coverage.out and coverage.html
make bench       # benchmarks with allocation measurements
make fuzz        # fuzz route matching for 10 seconds
```

`make` is optional: equivalent Go commands are in
[CONTRIBUTING.md](CONTRIBUTING.md). Benchmarks depend on the Go version, hardware
and workload; run them locally instead of relying on historical timing claims.

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
