# Contributing to Jano

Contributions in English or Portuguese are welcome. Follow our
[code of conduct](CODE_OF_CONDUCT.md) in all project interactions.

## Getting started

1. Fork the repository and clone your fork.
2. Install Go 1.22.5 or newer. Keep compatibility with the minimum in `go.mod`.
3. Create a branch with a descriptive name, such as `fix/route-parameters`.
4. Run `go test ./...` before making changes.

There are no external Go dependencies. Keep the library small and compatible
with `net/http`; follow the [architecture contracts](docs/architecture.md) and discuss changes to the public API, routing semantics or new
runtime dependencies in an issue before investing in a large implementation.
Documentation fixes and small bug fixes can go straight to a pull request.

## Code and tests

- Format Go code with `gofmt`; use idiomatic Go and document exported symbols.
- Keep each pull request focused on one problem. Avoid unrelated refactors.
- Test observable behavior through `httptest`, including relevant error cases.
- Add a regression test for a bug fix. Prefer table-driven tests for related cases.
- Keep tests deterministic and independent; avoid external services and sleeps.
- Update the README or examples whenever public behavior changes.
- Preserve the existing BSD-3-Clause license and copyright notices. Contributions
  are provided under the same license; no separate CLA is required by this project.

## Local checks

```sh
make check
```

Without Make, run:

```sh
gofmt -w $(find . -type f -name '*.go' -not -path './.git/*')
go vet ./...
go test -race ./...
go build ./...
```

`make check` rejects unformatted files without modifying them. `make fmt`
formats all Go files. Optional checks:

```sh
go test -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go test -run='^$' -bench=. -benchmem .
go test -run='^$' -fuzz=FuzzMatchRoute -fuzztime=10s .
```

Benchmark comparisons should include the command, Go version, OS/architecture
and CPU. Compare the same workload before and after a performance change.

## Pull requests

Explain the problem, the resulting behavior and how you verified it. Link a
related issue when available. A short imperative commit subject is sufficient,
for example `Fix example module imports`; Conventional Commit prefixes are
optional. The PR template provides a checklist.

CI checks formatting, `go vet`, race-enabled tests and builds on the minimum Go
version and the latest stable version. Maintainers review compatibility and
scope before merging. Review feedback should address the code respectfully.

## Bug reports and feature requests

Use the issue templates. Include a minimal reproducer, expected and actual
behavior, `go version`, and OS/architecture for bugs. For features, explain the
use case and alternatives. Do not include credentials or private data.
For vulnerabilities, follow [SECURITY.md](SECURITY.md).

## Maintainer release checklist

Before tagging a release, run `make check`, review API compatibility and describe
user-visible changes in the GitHub release notes. Use `vMAJOR.MINOR.PATCH` tags:
patches for compatible fixes, minor versions for compatible features, and major
versions for incompatible changes. Follow Go module major-version rules when
introducing v2 or later. Do not publish a release until CI passes.
