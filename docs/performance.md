# Performance

GSX is optimized around ahead-of-time code generation and direct writes to `io.Writer`.

## Design choices that matter

- Templates compile to plain Go functions.
- Generated renderers write directly to the provided writer.
- No runtime template parser runs on the production path.
- Layout slots compile to generated slot structs instead of generic maps.
- Same-package component and layout calls are inlined when safe, avoiding local slot closures and repeated helper calls.
- Escaping uses typed runtime helpers with fast no-escape paths for strings and byte slices.
- String writes avoid `[]byte(s)` fallback allocations for plain `io.Writer` implementations.
- Static HTML is emitted as direct string writes, not rebuilt through a node tree.

## Benchmark command

```bash
go test -bench=. -benchmem ./benchmarks
```

## Current results

Machine:
- OS: Linux amd64
- CPU: Intel(R) Core(TM) Ultra 9 275HX

Results captured on May 29, 2026:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkGSXSimple` | 143.9 | 496 | 4 |
| `BenchmarkGSXSimpleDiscard` | 43.07 | 0 | 0 |
| `BenchmarkGSXSimpleResponseRecorder` | 445.6 | 1456 | 12 |
| `BenchmarkGSXSimplePlainWriter` | 45.72 | 8 | 1 |
| `BenchmarkHTMLTemplateSimple` | 834.7 | 864 | 17 |
| `BenchmarkGomponentsSimple` | 744.2 | 1288 | 30 |
| `BenchmarkGSXList` | 10742 | 32752 | 10 |
| `BenchmarkGSXListDiscard` | 5219 | 0 | 0 |
| `BenchmarkGSXListResponseRecorder` | 13083 | 33712 | 18 |
| `BenchmarkGSXListPlainWriter` | 5288 | 8 | 1 |
| `BenchmarkGSXNestedLayouts` | 11224 | 32752 | 10 |
| `BenchmarkHTMLTemplateList` | 147580 | 84475 | 2424 |
| `BenchmarkGomponentsList` | 67384 | 112200 | 2437 |

## Reading the numbers

Current steady-state render behavior in this repo shows:
- GSX is roughly 5.8x faster than `html/template` on the simple page benchmark.
- GSX is roughly 5.2x faster than `gomponents` on the simple page benchmark.
- GSX is roughly 13.7x faster than `html/template` on the list benchmark.
- GSX is roughly 6.3x faster than `gomponents` on the list benchmark.
- GSX also allocates materially less in both benchmark groups.
- The nested-layout benchmark stays close to the plain list benchmark even with an extra forwarded named slot and wrapper layout layer.
- Writer choice matters: direct streaming to `io.Discard` or a plain writer avoids buffer growth allocations that are present in `bytes.Buffer` benchmarks.

## Caveats

Benchmark results depend on:
- Go version
- CPU and memory behavior
- output size
- component shape
- slot and closure usage

Simple components are close to the best-case path. Heavy slot composition and large loops still perform well, but output buffering can dominate allocations when rendering into growing buffers.

## Where to improve next

Likely future optimization targets:
- reducing writer growth pressure on very large responses
- optional generated size hints for buffered rendering
- more compile-time specialization for cross-package component calls
