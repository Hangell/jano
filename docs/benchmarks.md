# Routing benchmarks

These local results compare the previous router at `6b5f41c` with this working-tree
implementation. The same benchmark file was copied to a temporary checkout of
that commit, then run on both versions. Each result below is the median of three
samples. No external framework was benchmarked.

Environment: `go version go1.27.1 linux/amd64`, Linux/amd64, Intel Core i7-10510U.
The run is illustrative and was not an isolated laboratory measurement; CPU load,
thermal state and map iteration can affect timings. Raw output is in
[before](benchmark-before.txt) and [after](benchmark-after.txt).

```sh
go test -run='^$' -bench='Benchmark(JanoSimple|RouteCount)$' \
  -benchmem -benchtime=200ms -count=3 .
```

| Case | Before ns/op | After ns/op | After B/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `JanoSimple` | 939.90 | 81.43 | 0 | 0 |
| `RouteCount/static/10` | 4,247.00 | 94.40 | 0 | 0 |
| `RouteCount/parameter/10` | 6,489.00 | 2,521.00 | 992 | 8 |
| `RouteCount/static/100` | 31,604.00 | 75.53 | 0 | 0 |
| `RouteCount/parameter/100` | 37,476.00 | 1,374.00 | 992 | 8 |
| `RouteCount/static/1000` | 308,736.00 | 69.32 | 0 | 0 |
| `RouteCount/parameter/1000` | 428,777.00 | 1,574.00 | 992 | 8 |

The handlers are no-ops. Requests and response recorders are reused; registration
and snapshot compilation are outside the timed section. These numbers measure
route dispatch, not HTTP transport, JSON encoding, middleware, database queries,
or end-to-end application throughput. Static routes use the exact-path index and
allocate nothing in these workloads. Dynamic routes include request cloning and
parameter propagation through both PathValue and legacy context access.

`RouteCount` adds 10, 100 or 1,000 distinct routes and repeatedly requests the last
one. The old router traverses a map with variable iteration order, so its timings
and allocations vary between samples. The new router's exact-path index and segment
trie avoid that full-map search for these workloads.

Before making a performance claim, rerun the benchmark on the target environment,
collect enough samples, and compare with benchstat or another statistical tool.
Keep compatibility tests and real application workloads in the comparison. These
results do not establish parity or superiority over Gin or another framework.
