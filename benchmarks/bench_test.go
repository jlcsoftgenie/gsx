package benchmarks

import (
	"bytes"
	"html/template"
	"testing"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

var simpleTemplate = template.Must(template.New("simple").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}}</title></head><body><main><h1>{{.Title}}</h1><p>Fast, compiled HTML rendering.</p></main></body></html>`))
var listTemplate = template.Must(template.New("list").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}}</title></head><body><main><h1>{{.Title}}</h1><ul>{{range .Users}}<li class="user-row"><strong>{{.Name}}</strong><span>{{.Email}}</span></li>{{end}}</ul></main></body></html>`))

func benchUsers(n int) []User {
	users := make([]User, 0, n)
	for i := 0; i < n; i++ {
		users = append(users, User{Name: "User", Email: "user@example.com"})
	}
	return users
}

func BenchmarkGSXSimple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := RenderBenchSimple(&buf, "Bench"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTMLTemplateSimple(b *testing.B) {
	b.ReportAllocs()
	data := struct{ Title string }{Title: "Bench"}
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := simpleTemplate.Execute(&buf, data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGomponentsSimple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		node := h.Doctype(
			h.HTML(
				h.Head(
					h.Meta(h.Charset("utf-8")),
					h.TitleEl(g.Text("Bench")),
				),
				h.Body(
					h.Main(
						h.H1(g.Text("Bench")),
						h.P(g.Text("Fast, compiled HTML rendering.")),
					),
				),
			),
		)
		if err := node.Render(&buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGSXList(b *testing.B) {
	b.ReportAllocs()
	users := benchUsers(200)
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := RenderBenchList(&buf, "Bench", users); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGSXNestedLayouts(b *testing.B) {
	b.ReportAllocs()
	users := benchUsers(200)
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := RenderBenchNestedLayouts(&buf, "Bench", users); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTMLTemplateList(b *testing.B) {
	b.ReportAllocs()
	data := struct {
		Title string
		Users []User
	}{Title: "Bench", Users: benchUsers(200)}
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := listTemplate.Execute(&buf, data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGomponentsList(b *testing.B) {
	b.ReportAllocs()
	users := benchUsers(200)
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		node := h.Doctype(
			h.HTML(
				h.Head(
					h.Meta(h.Charset("utf-8")),
					h.TitleEl(g.Text("Bench")),
				),
				h.Body(
					h.Main(
						h.H1(g.Text("Bench")),
						h.Ul(g.Map(users, func(user User) g.Node {
							return h.Li(
								h.Class("user-row"),
								h.Strong(g.Text(user.Name)),
								h.Span(g.Text(user.Email)),
							)
						})),
					),
				),
			),
		)
		if err := node.Render(&buf); err != nil {
			b.Fatal(err)
		}
	}
}
