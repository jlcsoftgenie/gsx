package linter

import (
	"slices"
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

func TestLintPackageBasicWarnings(t *testing.T) {
	src := `package pages

component Card() {
  <img src="/x.png" />
  <button>Save</button>
  <raw html={trusted} />
}
`
	diags := lintSource(t, src)
	assertCodes(t, diags, "L001", "L002", "L003")
}

func TestLintPackageAccessibilityWarnings(t *testing.T) {
	src := `package pages

component FormPage() {
  <a href="/docs" target="_blank">Docs</a>
  <label for="missing">Missing</label>
  <input id="email" />
  <textarea />
  <select aria-label="Sort">
    <option>Latest</option>
  </select>
  <label>
    Name
    <input />
  </label>
}
`
	diags := lintSource(t, src)
	assertCodes(t, diags, "L009", "L010", "L010", "L011")
}

func TestLintPackageComponentSlotWarningAcrossImportedPackage(t *testing.T) {
	layouts := mustParseFile(t, "layouts.gsx", `package layouts

component Panel(title string) {
  <section>
    <h1>{title}</h1>
  </section>
}
`)
	page := mustParseFile(t, "page.gsx", `package pages

import layouts "example.com/app/layouts"

component Page() {
  <layouts.Panel title="Hello">
    <p>Unused child content</p>
  </layouts.Panel>
}
`)
	reg := compiler.NewRegistry()
	layoutsPkg, diags := compiler.BuildPackage("/repo/layouts", "example.com/app/layouts", []*ast.File{layouts})
	if len(diags) > 0 {
		t.Fatalf("layout build diagnostics: %+v", diags)
	}
	reg.Add(layoutsPkg)
	pagePkg, diags := compiler.BuildPackage("/repo/pages", "example.com/app/pages", []*ast.File{page})
	if len(diags) > 0 {
		t.Fatalf("page build diagnostics: %+v", diags)
	}
	reg.Add(pagePkg)
	compileDiags := compiler.ValidateAll(reg)
	if len(compileDiags) > 0 {
		t.Fatalf("compile diagnostics: %+v", compileDiags)
	}
	lintDiags := LintPackage(pagePkg, false)
	assertDiagnosticCodes(t, lintDiags, "L006")
}

func lintSource(t *testing.T, src string) []diagnosticsLite {
	t.Helper()
	file := mustParseFile(t, "test.gsx", src)
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	lintDiags := LintPackage(pkg, false)
	out := make([]diagnosticsLite, 0, len(lintDiags))
	for _, diag := range lintDiags {
		out = append(out, diagnosticsLite{Code: diag.Code, Message: diag.Message})
	}
	return out
}

type diagnosticsLite struct {
	Code    string
	Message string
}

func mustParseFile(t *testing.T, name, src string) *ast.File {
	t.Helper()
	file, diags := parser.ParseFile(name, src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	return file
}

func assertCodes(t *testing.T, diags []diagnosticsLite, want ...string) {
	t.Helper()
	got := make([]string, 0, len(diags))
	for _, diag := range diags {
		got = append(got, diag.Code)
	}
	slices.Sort(got)
	sortedWant := slices.Clone(want)
	slices.Sort(sortedWant)
	if !slices.Equal(got, sortedWant) {
		t.Fatalf("got lint codes %v, want %v", got, sortedWant)
	}
}

func assertDiagnosticCodes(t *testing.T, diags []diagnostics.Diagnostic, want ...string) {
	t.Helper()
	got := make([]string, 0, len(diags))
	for _, diag := range diags {
		got = append(got, diag.Code)
	}
	slices.Sort(got)
	sortedWant := slices.Clone(want)
	slices.Sort(sortedWant)
	if !slices.Equal(got, sortedWant) {
		t.Fatalf("got lint codes %v, want %v", got, sortedWant)
	}
}
