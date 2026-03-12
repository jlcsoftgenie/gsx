package parser

import (
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/ast"
)

func TestParseFile(t *testing.T) {
	src := `package pages

component Layout(title string) {
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

component Home(title string, users []User) {
  <Layout title={title}>
    if len(users) == 0 {
      <p>No users found.</p>
    } else {
      <ul>
        for _, user := range users {
          <li>{user.Name}</li>
        }
      </ul>
    }
  </Layout>
}
`
	file, diags := ParseFile("test.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if file.Package != "pages" {
		t.Fatalf("package = %q", file.Package)
	}
	if len(file.Components) != 2 {
		t.Fatalf("components = %d", len(file.Components))
	}
	if file.Components[1].Name != "Home" {
		t.Fatalf("component name = %q", file.Components[1].Name)
	}
}

func TestParseFileDeclarationStatements(t *testing.T) {
	src := `package pages

component Home(users []User) {
  count := len(users)
  const emptyLabel = "No users found."
  var first *User

  if count > 0 {
    first := &users[0]
    <p>{first.Name}</p>
  } else {
    <p>{emptyLabel}</p>
  }
}
`
	file, diags := ParseFile("test.gsx", src)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	body := file.Components[0].Body
	if len(body) < 4 {
		t.Fatalf("expected declaration nodes in body, got %d nodes", len(body))
	}
	if decl, ok := body[0].(*ast.Decl); !ok || decl.Code != "count := len(users)" {
		t.Fatalf("first node = %#v", body[0])
	}
	if decl, ok := body[1].(*ast.Decl); !ok || decl.Code != `const emptyLabel = "No users found."` {
		t.Fatalf("second node = %#v", body[1])
	}
	if decl, ok := body[2].(*ast.Decl); !ok || decl.Code != "var first *User" {
		t.Fatalf("third node = %#v", body[2])
	}
}

func TestParseFileRejectsReassignmentStatement(t *testing.T) {
	src := `package pages

component Home(users []User) {
  count := len(users)
  count = count + 1
  <p>{count}</p>
}
`
	_, diags := ParseFile("test.gsx", src)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics")
	}
	found := false
	for _, diag := range diags {
		if diag.Code == "P025" && diag.Message == "reassignment is not supported in template bodies; use := to declare a new variable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reassignment diagnostic, got %+v", diags)
	}
}
