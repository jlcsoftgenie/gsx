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
- Generated `Render...Buffer` helpers pre-grow `bytes.Buffer` values with generated size heuristics.

## Benchmark command

```bash
go test -bench=. -benchmem ./benchmarks
```

## Current results

Machine:
- OS: Linux amd64
- CPU: Intel(R) Core(TM) Ultra 9 275HX

Results captured on July 27, 2026 with Go 1.26.2:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkGSXSimple` | 217.4 | 496 | 4 |
| `BenchmarkGSXSimpleBuffer` | 153.8 | 272 | 2 |
| `BenchmarkGSXSimpleDiscard` | 50.34 | 0 | 0 |
| `BenchmarkGSXSimpleResponseRecorder` | 654.3 | 1456 | 12 |
| `BenchmarkGSXSimplePlainWriter` | 54.92 | 8 | 1 |
| `BenchmarkHTMLTemplateSimple` | 1164 | 864 | 17 |
| `BenchmarkGomponentsSimple` | 1047 | 1288 | 30 |
| `BenchmarkGSXList` | 15455 | 32752 | 10 |
| `BenchmarkGSXListBuffer` | 11578 | 24624 | 2 |
| `BenchmarkGSXListDiscard` | 6303 | 0 | 0 |
| `BenchmarkGSXListResponseRecorder` | 18244 | 33712 | 18 |
| `BenchmarkGSXListPlainWriter` | 6364 | 8 | 1 |
| `BenchmarkGSXNestedLayouts` | 15518 | 32752 | 10 |
| `BenchmarkHTMLTemplateList` | 202122 | 84474 | 2424 |
| `BenchmarkGomponentsList` | 93879 | 112200 | 2437 |

## Reading the numbers

Current steady-state render behavior in this repo shows:
- The normal streaming GSX API is roughly 5.4x faster than `html/template` on the simple page benchmark.
- The buffered GSX API is roughly 7.6x faster than `html/template` on the same fixture, with 2 allocations instead of 4.
- The normal streaming GSX API is roughly 13.1x faster than `html/template` on the list benchmark.
- The buffered GSX API is roughly 17.5x faster than `html/template` on the same list, with 2 allocations instead of 10.
- GSX is also faster and allocates materially less than `gomponents` in both benchmark groups.
- The nested-layout benchmark stays close to the plain list benchmark even with an extra forwarded named slot and wrapper layout layer.
- Writer choice matters: direct streaming to `io.Discard` or a plain writer avoids buffer growth allocations that are present in `bytes.Buffer` benchmarks.

## Buffered rendering

The normal generated API streams directly to `io.Writer` and is the best fit for
`http.ResponseWriter`. When a caller deliberately needs an in-memory document,
use the generated `Render<Component>Buffer` function:

```go
var buf bytes.Buffer
if err := RenderHomePageBuffer(&buf, title, users); err != nil {
	return err
}
html := buf.String()
```

These helpers call `buf.Grow` with a generated heuristic. It includes static
markup, literal attributes, small budgets for dynamic fields, safely inlined
local components, and top-level `range` loops over component parameters. It
never limits output or changes rendering semantics; an underestimated hint simply
lets `bytes.Buffer` grow normally.

## Caveats

Benchmark results depend on:
- Go version
- CPU and memory behavior
- output size
- component shape
- slot and closure usage

Simple components are close to the best-case path. Heavy slot composition and large loops still perform well. For a growing `bytes.Buffer`, the generated buffer API can reduce allocation pressure when its hint matches the rendered data shape.

## Where to improve next

Likely future optimization targets:
- more compile-time specialization for cross-package component calls
- dynamic output-size hints for known collection sizes
