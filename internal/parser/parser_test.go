package parser

import "testing"

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
