# CLI

The `gsx` CLI operates on directories recursively and discovers `.gsx` files package-by-package.

## Commands

### `gsx generate`

Parses templates, validates them, and writes generated `.gsx.go` files next to each source file.

```bash
gsx generate .
gsx generate ./examples ./benchmarks
```

Behavior:
- scans recursively for `.gsx`
- groups files by directory/package
- resolves imported GSX packages by Go import path when they are discoverable under the requested roots
- fails on parse or compile diagnostics
- stores package fingerprints in `.gsx/cache.json`
- skips regeneration for unchanged packages while still validating the full template graph
- writes only when generated output changed

### `gsx build`

Runs `generate`, then `go build ./...` in each requested root.

```bash
gsx build .
```

### `gsx check`

Runs parse, compile validation, linting, and format drift detection without writing files.

```bash
gsx check .
```

This is the intended CI command.

### `gsx lint`

Runs template lint rules without writing output.

```bash
gsx lint .
```

Current built-in rules include checks for:
- missing `img` alt text
- missing `button` type
- suspicious raw HTML usage
- invalid block content inside `<p>`
- unused child content passed to components that do not render slots
- boolean `aria-*` attributes without explicit values
- `target="_blank"` anchors missing `rel="noopener noreferrer"`
- form controls without an associated label or ARIA label
- `label for="..."` references that do not match any local `id`

### `gsx fmt`

Formats `.gsx` files deterministically.

```bash
gsx fmt --write .
gsx fmt --check .
```

Flags:
- `--write`: write formatted files in place
- `--check`: exit non-zero if any file would change

If no flags are passed, `fmt` writes in place.

### `gsx watch`

Runs `generate` in an `fsnotify` loop whenever relevant files change.

```bash
gsx watch .
gsx watch --build .
gsx watch --build --command 'go test ./...' .
```

Flags:
- `--interval`: debounce interval for coalescing rapid file events, default `250ms`
- `--build`: run `build` instead of `generate`
- `--command`: shell command to run after a successful cycle

In `--build` mode, `watch` also reacts to `.go`, `go.mod`, `go.sum`, `go.work`, and `go.work.sum` changes.

### `gsx init`

Creates a starter GSX project in the target directory and generates the initial `.gsx.go` output.

```bash
gsx init --module example.com/myapp ./myapp
```

Flags:
- `--module`: module path to create when `go.mod` is missing
- `--package`: package name for starter files, default `main`
- `--force`: overwrite starter files

### `gsx version`

Prints the CLI version.

```bash
gsx version
```

## `go generate` integration

GSX works cleanly with `go generate`:

```go
//go:generate go run github.com/jlcsoftgenie/gsx/cmd/gsx generate .
```

That keeps template generation close to the package that owns the `.gsx` files.

## Diagnostics

Compiler, linter, and formatter diagnostics include:
- filename
- line
- column
- message
- optional note
- code frame when the source file is available

Typical output shape:

```text
path/to/file.gsx:12:7: missing required prop "title" on component Layout [C012]
   12 |   <Layout>
      |       ^
```
