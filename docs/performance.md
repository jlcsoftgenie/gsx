# Performance

GSX is optimized around ahead-of-time code generation and direct writes to `io.Writer`.

## Design choices that matter

- Templates compile to plain Go functions.
- Generated renderers write directly to the provided writer.
- No runtime template parser runs on the production path.
- Layout slots compile to generated slot structs instead of generic maps.
- Slot forwarding paths reuse parent slot functions directly when possible instead of allocating new forwarding closures.
- Escaping uses a small runtime helper with chunked writes.
- Static HTML is emitted as direct string writes, not rebuilt through a node tree.

## Benchmark command

```bash
go test -bench=. -benchmem ./benchmarks
```

## Current results

Machine:
- OS: Linux amd64
- CPU: Intel(R) Core(TM) i7-10700KF CPU @ 3.80GHz

Results captured on March 6, 2026:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkGSXSimple` | 486.5 | 528 | 6 |
| `BenchmarkHTMLTemplateSimple` | 2027 | 896 | 22 |
| `BenchmarkGomponentsSimple` | 1789 | 1288 | 30 |
| `BenchmarkGSXList` | 42191 | 39184 | 412 |
| `BenchmarkGSXNestedLayouts` | 42117 | 39216 | 414 |
| `BenchmarkHTMLTemplateList` | 370383 | 106869 | 3629 |
| `BenchmarkGomponentsList` | 149412 | 112201 | 2437 |

## Reading the numbers

Current steady-state render behavior in this repo shows:
- GSX is roughly 4.1x faster than `html/template` on the simple page benchmark.
- GSX is roughly 3.7x faster than `gomponents` on the simple page benchmark.
- GSX is roughly 8.8x faster than `html/template` on the list benchmark.
- GSX is roughly 3.5x faster than `gomponents` on the list benchmark.
- GSX also allocates materially less in both benchmark groups.
- The nested-layout benchmark stays close to the plain list benchmark even with an extra forwarded named slot and wrapper layout layer.

## Caveats

Benchmark results depend on:
- Go version
- CPU and memory behavior
- output size
- component shape
- slot and closure usage

Simple components are close to the best-case path. Heavy slot composition and large loops still perform well, but will allocate more because slot closures and large output buffers cost something.

## Where to improve next

Likely future optimization targets:
- tighter typed fast paths for common scalar expression rendering
- reducing writer growth pressure on very large responses
- more compile-time specialization for repeated expression patterns
