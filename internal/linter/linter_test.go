package linter

import (
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

func TestLintPackage(t *testing.T) {
	src := `package pages

component Card() {
  <img src="/x.png" />
  <button>Save</button>
  <raw html={trusted} />
}
`
	file, diags := parser.ParseFile("card.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	lintDiags := LintPackage(pkg, false)
	if len(lintDiags) != 3 {
		t.Fatalf("got %d lint diagnostics", len(lintDiags))
	}
}
