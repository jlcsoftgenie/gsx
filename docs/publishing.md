# Publishing GSX

GSX is a normal Go module. Publishing it is mostly a matter of pushing the repository to GitHub under the module path declared in `go.mod`:

```text
github.com/jlcsoftgenie/gsx
```

## 1. Create the GitHub repository

Create a repository named `gsx` under the `jlcsoftgenie` account.

The local source tree is already configured for that path:

- `go.mod` uses `module github.com/jlcsoftgenie/gsx`
- generated code imports `github.com/jlcsoftgenie/gsx/runtime`
- the VS Code extension metadata points at the same repository

## 2. Push the repository

From the repository root:

```bash
git init
git add .
git commit -m "Initial GSX release"
git branch -M main
git remote add origin git@github.com:jlcsoftgenie/gsx.git
git push -u origin main
```

If you prefer HTTPS:

```bash
git remote add origin https://github.com/jlcsoftgenie/gsx.git
```

## 3. Tag a release

For Go modules, an annotated semver tag is enough for consumers and pkg.go.dev:

```bash
git tag -a v0.2.0 -m "GSX v0.2.0"
git push origin v0.2.0
```

After the tag is reachable on GitHub, users can install the CLI with:

```bash
go install github.com/jlcsoftgenie/gsx/cmd/gsx@v0.2.0
```

Or depend on the module with:

```bash
go get github.com/jlcsoftgenie/gsx@v0.2.0
```

## 4. Verify the public module

From another directory:

```bash
mkdir /tmp/gsx-smoke
cd /tmp/gsx-smoke
go mod init example.com/smoke
go install github.com/jlcsoftgenie/gsx/cmd/gsx@v0.2.0
```

Then scaffold a starter app:

```bash
gsx init --module example.com/smoke-app .
go test ./...
```

## 5. Update downstream repos

Any local repos that currently use a placeholder path or local replace should be updated.

Example for `ylloyds`:

```go
require github.com/jlcsoftgenie/gsx v0.2.0
```

During local development you can still keep:

```go
replace github.com/jlcsoftgenie/gsx => ../gsx
```

Remove that `replace` before CI or production builds if they should fetch the tagged GitHub module instead of a sibling checkout.
