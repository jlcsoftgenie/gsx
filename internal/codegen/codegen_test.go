package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

func TestGenerateFile(t *testing.T) {
	src := `package pages

component Layout(title string) {
  <html>
    <head><title>{title}</title><slot name="head" /></head>
    <body><slot /></body>
  </html>
}

component Home(title string) {
  <Layout title={title}>
    <fragment slot="head"><meta name="description" content={title} /></fragment>
    <main><h1>{title}</h1></main>
  </Layout>
}
`
	file, diags := parser.ParseFile("pages.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"func RenderLayout", "func RenderLayoutBuffer", "type GSXLayoutSlots struct", "func RenderLayoutWithSlots", "func RenderLayoutBufferWithSlots", "return renderHome", "WriteEscaped", `"bytes"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
}

func TestGenerateFileCrossPackageComponentCall(t *testing.T) {
	layoutSrc := `package shared

component Shell(title string) {
  <html>
    <head><title>{title}</title><slot name="head" /></head>
    <body><slot /></body>
  </html>
}
`
	pageSrc := `package app

import shared "example.com/app/shared"

component Home(title string) {
  <shared.Shell title={title}>
    <fragment slot="head"><meta name="description" content={title} /></fragment>
    <main><h1>{title}</h1></main>
  </shared.Shell>
}
`
	layoutFile, diags := parser.ParseFile("shared/shell.gsx", layoutSrc)
	if len(diags) > 0 {
		t.Fatalf("layout parse diagnostics: %+v", diags)
	}
	pageFile, diags := parser.ParseFile("app/home.gsx", pageSrc)
	if len(diags) > 0 {
		t.Fatalf("page parse diagnostics: %+v", diags)
	}
	reg := compiler.NewRegistry()
	sharedPkg, diags := compiler.BuildPackage("shared", "example.com/app/shared", []*ast.File{layoutFile})
	if len(diags) > 0 {
		t.Fatalf("shared build diagnostics: %+v", diags)
	}
	appPkg, diags := compiler.BuildPackage("app", "example.com/app/app", []*ast.File{pageFile})
	if len(diags) > 0 {
		t.Fatalf("app build diagnostics: %+v", diags)
	}
	reg.Add(sharedPkg)
	reg.Add(appPkg)
	if diags := compiler.ValidateAll(reg); len(diags) > 0 {
		t.Fatalf("validation diagnostics: %+v", diags)
	}
	out, err := GenerateFile(appPkg, pageFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`"example.com/app/shared"`,
		"shared.RenderShellWithSlots",
		"shared.GSXShellSlots",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
}

func TestGenerateFileForwardedSlotsReuseParentSlotFuncs(t *testing.T) {
	src := `package pages

component BaseLayout(title string) {
  <html>
    <head><title>{title}</title><slot name="head" /></head>
    <body><slot /></body>
  </html>
}

component Shell(title string) {
  <BaseLayout title={title}>
    <slot name="head" />
    <section class="shell"><slot /></section>
  </BaseLayout>
}
`
	file, diags := parser.ParseFile("pages.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`<section class=\"shell\">`,
		"__gsx_slots.Head",
		"__gsx_slots.Default",
		"__gsx_buf.Grow(",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
	for _, notWant := range []string{
		"title := title",
		"Head: __gsx_slots.Head,",
		"Default: func(w gsxio.Writer) error {",
	} {
		if strings.Contains(text, notWant) {
			t.Fatalf("generated output should not contain %q\n%s", notWant, text)
		}
	}
}

func TestGenerateFileUsesParameterRangeInBufferHint(t *testing.T) {
	src := `package pages

component UserRow(name string) {
  <li>{name}</li>
}

component Users(names []string) {
  <ul>
    for _, name := range names {
      <UserRow name={name} />
    }
  </ul>
}
`
	file, diags := parser.ParseFile("pages.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "len(names)*") {
		t.Fatalf("generated output missing parameter range buffer hint\n%s", out)
	}
}

func TestGenerateFileCoalescesAdjacentStaticHTMLWrites(t *testing.T) {
	src := `package pages

component Home(title string) {
  <main class="hero"><h1>Hello</h1><p>{title}</p><footer>Done</footer></main>
}
`
	file, diags := parser.ParseFile("pages.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(".", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, `WriteString(w, "<main class=\"hero\"><h1>Hello</h1><p>")`) {
		t.Fatalf("expected coalesced opening literal write\n%s", text)
	}
	if !strings.Contains(text, `WriteString(w, "</p><footer>Done</footer></main>")`) {
		t.Fatalf("expected coalesced trailing literal write\n%s", text)
	}
}

func TestGenerateFileUsesTypedWritersWhenTypeInfoIsAvailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/typed\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `package pages

component Home(title string, count int, active bool) {
  <main data-count={count} data-active={active}>
    <h1>{title}</h1>
    <p>{count}</p>
    <p>{active}</p>
  </main>
}
`
	file, diags := parser.ParseFile(filepath.Join(dir, "home.gsx"), src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.BuildPackage(dir, "example.com/typed", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"WriteAttrInt64",
		"WriteAttrBool",
		"WriteEscapedString",
		"WriteInt64",
		"WriteBool",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
}

func TestGenerateFileUsesTypedByteWritersWhenTypeInfoIsAvailable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/bytes\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `package pages

component Home(label []byte) {
  <main data-label={label}>{label}</main>
}
`
	file, diags := parser.ParseFile(filepath.Join(dir, "home.gsx"), src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.BuildPackage(dir, "example.com/bytes", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"WriteAttrBytes",
		"WriteEscapedBytes",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "string(label)") {
		t.Fatalf("generated output should not convert []byte expression to string\n%s", text)
	}
}

func TestGenerateFileSupportsLocalDeclarationStatements(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/localdecl\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `package pages

component Home(users []string) {
  count := len(users)
  const emptyLabel = "No users"
  if count == 0 {
    <p>{emptyLabel}</p>
  } else {
    <p>{count}</p>
  }
}
`
	file, diags := parser.ParseFile(filepath.Join(dir, "home.gsx"), src)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.BuildPackage(dir, "example.com/localdecl", []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	out, err := GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"count := len(users)",
		`const emptyLabel = "No users"`,
		"WriteEscapedString(w, string(emptyLabel))",
		"WriteInt64(w, int64(count))",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated output missing %q\n%s", want, text)
		}
	}
}
