package gsx_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/codegen"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

func TestGeneratedTemplatesCompileAndRender(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	templateSource := `package pages

component BaseLayout(title string) {
  <!doctype html>
  <html>
    <head>
      <title>{title}</title>
      <slot name="head" />
    </head>
    <body>
      <slot />
    </body>
  </html>
}

component Wrapper(title string) {
  <BaseLayout title={title}>
    <slot name="head" />
    <section class="wrapper">
      <slot />
    </section>
  </BaseLayout>
}

component HomePage(data HomeData) {
  <Wrapper title={data.Title}>
    <fragment slot="head">
      <meta name="description" content={data.Description} />
    </fragment>
    <p>{data.Description}</p>
  </Wrapper>
}
`
	typesSource := `package pages

type HomeData struct {
  Title string
  Description string
}
`
	testSource := `package pages

import (
  "net/http/httptest"
  "strings"
  "testing"
)

func TestRenderHomePage(t *testing.T) {
  rec := httptest.NewRecorder()
  err := RenderHomePage(rec, HomeData{Title: "Welcome", Description: "Hello <world>"})
  if err != nil {
    t.Fatal(err)
  }
  body := rec.Body.String()
  if !strings.Contains(body, "<meta name=\"description\" content=\"Hello &lt;world&gt;\" />") {
    t.Fatalf("missing escaped head slot: %s", body)
  }
  if !strings.Contains(body, "<section class=\"wrapper\"><p>Hello &lt;world&gt;</p></section>") {
    t.Fatalf("missing wrapped body: %s", body)
  }
}
`
	pagesDir := filepath.Join(tmp, "pages")
	if err := os.MkdirAll(pagesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(moduleText(repoRoot)), 0o644); err != nil {
		t.Fatal(err)
	}
	copyRepoGoSum(t, repoRoot, tmp)
	if err := os.WriteFile(filepath.Join(pagesDir, "pages.gsx"), []byte(templateSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pagesDir, "types.go"), []byte(typesSource), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pagesDir, "pages_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}
	file, diags := parser.ParseFile(filepath.Join(pagesDir, "pages.gsx"), templateSource)
	if len(diags) > 0 {
		t.Fatalf("parse diagnostics: %+v", diags)
	}
	pkg, diags := compiler.CompilePackage(pagesDir, []*ast.File{file})
	if len(diags) > 0 {
		t.Fatalf("compile diagnostics: %+v", diags)
	}
	generated, err := codegen.GenerateFile(pkg, file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pagesDir, "pages.gsx.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	runGoModTidy(t, tmp)
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %v\n%s", err, out)
	}
}

func TestCLIGenerateCrossPackageTemplates(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	sharedTemplate := `package shared

component Shell(title string) {
  <html>
    <head><title>{title}</title><slot name="head" /></head>
    <body><slot /></body>
  </html>
}
`
	appTemplate := `package app

import shared "example.com/integration/shared"

component HomePage(title string, description string) {
  <shared.Shell title={title}>
    <fragment slot="head">
      <meta name="description" content={description} />
    </fragment>
    <p>{description}</p>
  </shared.Shell>
}
`
	appTest := `package app

import (
  "net/http/httptest"
  "strings"
  "testing"
)

func TestCrossPackageRender(t *testing.T) {
  rec := httptest.NewRecorder()
  if err := RenderHomePage(rec, "Hello", "Cross package"); err != nil {
    t.Fatal(err)
  }
  body := rec.Body.String()
  if !strings.Contains(body, "<meta name=\"description\" content=\"Cross package\" />") {
    t.Fatalf("missing imported head slot: %s", body)
  }
  if !strings.Contains(body, "<p>Cross package</p>") {
    t.Fatalf("missing body content: %s", body)
  }
}
`
	sharedDir := filepath.Join(tmp, "shared")
	appDir := filepath.Join(tmp, "app")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(moduleText(repoRoot)), 0o644); err != nil {
		t.Fatal(err)
	}
	copyRepoGoSum(t, repoRoot, tmp)
	if err := os.WriteFile(filepath.Join(sharedDir, "shell.gsx"), []byte(sharedTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "home.gsx"), []byte(appTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "home_test.go"), []byte(appTest), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "github.com/jlcsoftgenie/gsx/cmd/gsx", "generate", ".")
	cmd.Dir = tmp
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gsx generate failed: %v\n%s", err, out)
	}
	runGoModTidy(t, tmp)
	cmd = exec.Command("go", "test", "./...")
	cmd.Dir = tmp
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test failed: %v\n%s", err, out)
	}
}

func moduleText(repoRoot string) string {
	return `module example.com/integration

go 1.23.0

require github.com/jlcsoftgenie/gsx v0.0.0

replace github.com/jlcsoftgenie/gsx => ` + repoRoot + `
`
}

func runGoModTidy(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}
}

func copyRepoGoSum(t *testing.T, repoRoot, dir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read repo go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), data, 0o644); err != nil {
		t.Fatalf("write temp go.sum: %v", err)
	}
}
