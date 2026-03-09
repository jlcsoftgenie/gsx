# gsx

`gsx` is an ahead-of-time, JSX-like templating system for Go server-side HTML rendering.

It is built for teams that want:
- HTML-first authoring
- Go expressions and control flow
- ahead-of-time compilation into `.go`
- streaming renders to `io.Writer`
- safe escaping by default
- built-in layouts and slots
- deterministic formatting and linting
- a small runtime and predictable output

The compiler parses `.gsx` files, validates component calls and slot usage, and emits Go render functions that write directly to a writer.

## Install

Library module:

```bash
go get github.com/jlcsoftgenie/gsx@latest
```

CLI:

```bash
go install github.com/jlcsoftgenie/gsx/cmd/gsx@latest
```

## Status

This repository contains a complete, working implementation with:
- compiler/parser/code generator
- small runtime package
- CLI: `generate`, `build`, `check`, `lint`, `fmt`, `watch`, `init`, `version`
- formatter and linter
- VS Code extension with syntax highlighting, snippets, hover, formatting, diagnostics, and language-server completions
- layouts and named slots
- examples
- tests and integration coverage
- benchmarks against `html/template` and `gomponents`

Current intentional constraints:
- no runtime template parsing in the production path
- no client runtime or hydration
- no generic attribute spread, by design
- imported GSX component packages must be discoverable during `generate`/`check`

## Quick Look

```gsx
package pages

component BaseLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <header><slot name="header" /></header>
      <main><slot /></main>
    </body>
  </html>
}

component HomePage(title string, users []User) {
  <BaseLayout title={title}>
    <div slot="header">
      <h1>{title}</h1>
    </div>

    if len(users) == 0 {
      <p>No users found.</p>
    } else {
      <ul>
        for _, user := range users {
          <li>{user.Name}</li>
        }
      </ul>
    }
  </BaseLayout>
}
```

Generated public API:

```go
func RenderHomePage(w io.Writer, title string, users []User) error
func RenderHomePageWithSlots(w io.Writer, slots GSXHomePageSlots, title string, users []User) error
```

## Why GSX Exists

`html/template` is safe and standard, but it stays string/template oriented and does its work at runtime.
GSX takes a different tradeoff:
- author templates in an HTML-shaped language
- keep expressions and control flow in Go
- resolve component contracts ahead of time
- render with direct writes instead of interpreting a template AST per request

That makes it fit well for:
- net/http apps
- admin dashboards
- HTMX responses
- content-heavy pages
- server-rendered internal tools

## Syntax Summary

File structure:
- `package` declaration is required
- optional Go `import` declarations are supported
- one or more `component` declarations per file

Supported template features:
- HTML-like elements and self-closing tags
- `{expr}` for Go expressions
- `if / else if / else`
- `for` loops
- comments: `<!-- -->`
- doctype nodes
- fragments: `<fragment>...</fragment>`
- layouts and slots: `<slot />`, `<slot name="head" />`
- trusted raw HTML: `<raw html={runtime.HTML(...)} />`
- imported GSX components with standard Go imports and aliases, for example `<shared.Panel />`

See [docs/syntax.md](docs/syntax.md) for the full language and grammar.

## Slots And Layouts

Layouts are just components that render `<slot>` outlets.
Named slots are passed with a `slot="name"` attribute on direct child nodes.
Wrapper components can forward parent slot content to nested layouts with direct child `<slot />` and `<slot name="..." />` nodes.
Imported GSX layout packages work the same way; the compiler resolves them through normal Go import paths.

## Safety Model

Default behavior:
- text expressions are HTML-escaped
- dynamic attribute values are HTML-escaped
- boolean attributes are emitted safely
- indentation-only whitespace is removed predictably

Trusted raw HTML is explicit:

```gsx
<raw html={runtime.HTML("<strong>trusted</strong>")} />
```

Normal `{expr}` output never bypasses escaping, even if the value is `runtime.HTML`.

See [docs/escaping.md](docs/escaping.md).

## CLI

Build and generate:

```bash
gsx generate .
gsx build .
gsx check .
gsx watch --build .
gsx init --module example.com/myapp ./myapp
```

Formatting and linting:

```bash
gsx fmt --write .
gsx fmt --check .
gsx lint .
```

See [docs/cli.md](docs/cli.md).

## Publishing

This repository is configured for the GitHub module path `github.com/jlcsoftgenie/gsx`.
See [docs/publishing.md](docs/publishing.md) for the exact push/tag flow.

## Examples

- [`examples/basic`](examples/basic): basic page, nested components, list rendering
- [`examples/layouts`](examples/layouts): named slots and nested layout forwarding
- [`examples/crosspkg`](examples/crosspkg): cross-package component imports and layouts
- [`examples/webserver`](examples/webserver): standard `net/http` server with multiple routes
- [`examples/htmx`](examples/htmx): HTMX-friendly page and partial rendering
- [`examples/admin`](examples/admin): dashboard-style page with forms and metrics

## Benchmarks

Measured with `go test -bench=. -benchmem ./benchmarks` on Linux amd64, Intel i7-10700KF:

| Benchmark | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkGSXSimple` | 486.5 | 528 | 6 |
| `BenchmarkHTMLTemplateSimple` | 2027 | 896 | 22 |
| `BenchmarkGomponentsSimple` | 1748 | 1288 | 30 |
| `BenchmarkGSXList` | 42191 | 39184 | 412 |
| `BenchmarkGSXNestedLayouts` | 42117 | 39216 | 414 |
| `BenchmarkHTMLTemplateList` | 370383 | 106869 | 3629 |
| `BenchmarkGomponentsList` | 149412 | 112201 | 2437 |

See [docs/performance.md](docs/performance.md).

## Repository Layout

- [`cmd/gsx`](cmd/gsx): CLI
- [`internal/lexer`](internal/lexer): top-level lexer
- [`internal/parser`](internal/parser): parser and template-body scanner
- [`internal/compiler`](internal/compiler): semantic validation and package model
- [`internal/codegen`](internal/codegen): Go source generator
- [`internal/formatter`](internal/formatter): deterministic formatter
- [`internal/linter`](internal/linter): lint rules and diagnostics
- [`runtime`](runtime): escaping and write helpers used by generated code
- [`benchmarks`](benchmarks): benchmark suite
- [`examples`](examples): runnable examples
- [`editors/vscode/gsx`](editors/vscode/gsx): VS Code extension for `.gsx`
- [`docs`](docs): language, architecture, CLI, safety, performance, migration notes

## Generated Output

Generated files:
- live next to the `.gsx` source file
- are named `*.gsx.go`
- expose `Render<Component>` wrappers
- expose `Render<Component>WithSlots` and `GSX<Component>Slots` for advanced composition
- keep a small internal render signature for local component calls

Example:

```go
func RenderUsersPage(w io.Writer, title string, users []User) error
func RenderUsersPageWithSlots(w io.Writer, slots GSXUsersPageSlots, title string, users []User) error
```

## Development

Run the full suite:

```bash
go test ./...
go test -bench=. -benchmem ./benchmarks
go run ./cmd/gsx check .
```

Regenerate templates after editing examples or benchmarks:

```bash
go run ./cmd/gsx generate ./examples ./benchmarks
```

## Migration

For migration guidance from `html/template`, see [docs/migration-from-html-template.md](docs/migration-from-html-template.md).

## License

MIT. See [LICENSE](LICENSE).
