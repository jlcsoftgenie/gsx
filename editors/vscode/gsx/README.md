# GSX Language Tools

VS Code support for `.gsx` templates.

Quick syntax reference:
- `package` and `import` work like Go
- `component Name(args...) { ... }` defines a typed GSX component
- lowercase tags render HTML-like elements
- capitalized tags call GSX components
- `{expr}` embeds Go expressions
- `if` / `else` / `for` use Go syntax
- `<slot />`, `<slot name="...">`, and `slot="name"` support layouts
- `<fragment>` groups children without rendering a wrapper
- `<raw html={runtime.HTML(...)} />` is the explicit raw HTML escape hatch

Full cheat sheet:
- [`docs/cheatsheet.md`](../../../docs/cheatsheet.md)

Features:
- syntax highlighting for GSX tags, components, attributes, declarations, expressions, and comments
- semantic tokens for Go-like regions such as component signatures, expressions, and control-flow headers
- language configuration for brackets and auto-closing pairs
- snippets for components, layouts, conditionals, loops, named slots, and raw HTML
- language-server powered completion support for:
  - HTML tags
  - GSX built-in tags (`fragment`, `slot`, `raw`)
  - local component names
  - common attributes
  - same-module imported GSX component aliases and components
- hover documentation for:
  - GSX syntax keywords and built-in tags
  - component signatures
  - component parameters and parameter usages
  - component prop names at call sites
  - inferred `for ... range` loop variables
  - selector fields resolved from local Go `struct` types
  - local Go type names used in component signatures
- document symbols for GSX components, parameters, and slot outlets in the outline and breadcrumbs
- go to definition for local and same-module imported GSX component tags
- find references for local and same-module imported GSX components
- rename support for GSX component declarations and usages within the current module
- signature help for component props while typing inside component tags
- code actions / quick fixes for common GSX diagnostics like missing `alt`, missing `button type`, ARIA boolean values, and formatting drift
  plus `target="_blank"` anchors missing `rel="noopener noreferrer"`
- optional `gopls`-backed hover and completion inside single-line `{...}` Go expression regions
- document formatting through the `gsx fmt` CLI
- diagnostics through the `gsx check` CLI

## Commands

- `GSX: Format Document`
- `GSX: Run Check`

## Configuration

- `gsx.command`: executable used to invoke GSX CLI operations; leave empty to auto-detect the local workspace CLI first, then fall back to `gsx` on `PATH`
- `gsx.commandArgs`: arguments inserted before the subcommand when `gsx.command` is set
- `gsx.checkOnOpen`: run diagnostics when opening a GSX file
- `gsx.checkOnSave`: run diagnostics when saving a GSX file
- `gsx.diagnosticDebounceMs`: debounce delay before running diagnostics
- `gsx.goplsCommand`: optional explicit path to `gopls`; leave empty for auto-detection

Example explicit configuration:

```json
{
  "gsx.command": "go",
  "gsx.commandArgs": ["run", "/absolute/path/to/cmd/gsx"]
}
```

## Development

```bash
cd editors/vscode/gsx
npm install
npm run compile
```

To run the extension locally from the full GSX repo:
1. Open the repo root in VS Code.
2. Press `F5`.
3. Choose `GSX VS Code Extension`.
4. Open a `.gsx` file in the new Extension Development Host window.

To run it from just the extension folder:
1. Open `editors/vscode/gsx` in VS Code.
2. Press `F5`.
3. Choose `GSX VS Code Extension`.
4. Open a `.gsx` file in the new Extension Development Host window.

## Install From VSIX

Build the packaged extension:

```bash
cd editors/vscode/gsx
npm run package
```

Install or update it in your main VS Code:

```bash
code --install-extension gsx-language-tools-0.4.0.vsix --force
```

Then run `Developer: Reload Window`.

If the installed extension behaves differently from the Extension Development Host:
1. Check your normal VS Code settings for old `gsx.command` or `gsx.commandArgs` values.
2. Clear them unless you intentionally want to override auto-detection.
3. Reinstall the VSIX with `--force` so VS Code picks up the new package version and grammar files.

## Notes

- Imported component completion currently resolves same-module Go import paths by reading the nearest `go.mod` and scanning `.gsx` files in the target package directory.
- Formatting and diagnostics auto-detect the local GSX CLI from the workspace when possible.
- `gsx check` diagnostics are disk-backed, so the extension reports the saved file state.
