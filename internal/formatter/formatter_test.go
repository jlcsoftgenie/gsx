package formatter

import "testing"

func TestFormat(t *testing.T) {
	src := "package pages\ncomponent Home(title string){<div class=\"hero\"><h1>{ title }</h1></div>}"
	out, err := Format("home.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	want := `package pages

component Home(title string) {
  <div class="hero">
    <h1>{title}</h1>
  </div>
}
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestFormatPreservesInlineTextSpacing(t *testing.T) {
	src := "package pages\ncomponent Home(name string){<p>Hello {name}</p>}"
	out, err := Format("home.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	want := `package pages

component Home(name string) {
  <p>Hello {name}</p>
}
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestFormatExpandsNestedMarkup(t *testing.T) {
	src := "package pages\ncomponent Card(metric Metric){<article class=\"metric-card\"><h2>{metric.Value}</h2><p>{metric.Label}</p></article>}"
	out, err := Format("card.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	want := `package pages

component Card(metric Metric) {
  <article class="metric-card">
    <h2>{metric.Value}</h2>
    <p>{metric.Label}</p>
  </article>
}
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestFormatDeclarationStatements(t *testing.T) {
	src := "package pages\ncomponent Home(users []User){count:=len(users)\nconst emptyLabel=\"No users\"\nif count == 0 {<p>{emptyLabel}</p>}}"
	out, err := Format("home.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	want := `package pages

component Home(users []User) {
  count := len(users)
  const emptyLabel = "No users"
  if count == 0 {
    <p>{emptyLabel}</p>
  }
}
`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
