package formatter

import (
	"bytes"
	"fmt"
	goast "go/ast"
	"go/format"
	goparser "go/parser"
	"go/token"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/parser"
)

type printer struct {
	buf    bytes.Buffer
	indent int
}

func Format(path, src string) (string, error) {
	file, diags := parser.ParseFile(path, src)
	if len(diags) > 0 {
		return "", fmt.Errorf("cannot format %s: parse failed", path)
	}
	p := &printer{}
	p.line("package %s", file.Package)
	if len(file.Imports) > 0 {
		p.line("")
		if len(file.Imports) == 1 {
			imp := file.Imports[0]
			if imp.Alias != "" {
				p.line("import %s %q", imp.Alias, imp.Path)
			} else {
				p.line("import %q", imp.Path)
			}
		} else {
			p.line("import (")
			p.indent++
			for _, imp := range file.Imports {
				if imp.Alias != "" {
					p.line("%s %q", imp.Alias, imp.Path)
				} else {
					p.line("%q", imp.Path)
				}
			}
			p.indent--
			p.line(")")
		}
	}
	for _, comp := range file.Components {
		p.line("")
		p.printComponent(comp)
	}
	return strings.TrimSpace(p.buf.String()) + "\n", nil
}

func IsFormatted(path, src string) (bool, error) {
	out, err := Format(path, src)
	if err != nil {
		return false, err
	}
	return out == src, nil
}

func (p *printer) printComponent(comp *ast.ComponentDecl) {
	params := make([]string, 0, len(comp.Params))
	for _, param := range comp.Params {
		params = append(params, fmt.Sprintf("%s %s", param.Name, param.Type))
	}
	p.line("component %s(%s) {", comp.Name, strings.Join(params, ", "))
	p.indent++
	p.printNodes(comp.Body)
	p.indent--
	p.line("}")
}

func (p *printer) printNodes(nodes []ast.Node) {
	for _, node := range nodes {
		p.printNode(node)
	}
}

func (p *printer) printNode(node ast.Node) {
	switch n := node.(type) {
	case *ast.Text:
		text := strings.TrimRight(n.Value, "\n")
		if text == "" {
			return
		}
		for _, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			p.line(trimmed)
		}
	case *ast.Expr:
		p.line("{%s}", strings.TrimSpace(n.Code))
	case *ast.Decl:
		p.line("%s", formatGoStatement(n.Code))
	case *ast.Comment:
		p.line("<!--%s-->", strings.TrimSpace(n.Value))
	case *ast.Doctype:
		p.line("<!%s>", strings.TrimSpace(n.Value))
	case *ast.If:
		for i, branch := range n.Branches {
			prefix := "if"
			if i > 0 {
				prefix = "else if"
			}
			p.line("%s %s {", prefix, strings.TrimSpace(branch.Cond))
			p.indent++
			p.printNodes(branch.Body)
			p.indent--
			p.line("}")
		}
		if len(n.ElseBody) > 0 {
			p.line("else {")
			p.indent++
			p.printNodes(n.ElseBody)
			p.indent--
			p.line("}")
		}
	case *ast.For:
		p.line("for %s {", strings.TrimSpace(n.Header))
		p.indent++
		p.printNodes(n.Body)
		p.indent--
		p.line("}")
	case *ast.Element:
		p.printElement(n)
	}
}

func (p *printer) printElement(elem *ast.Element) {
	if elem.Name == "fragment" && len(elem.Attributes) == 0 && isInlineChildren(elem.Children) {
		p.line("<fragment>%s</fragment>", p.inlineChildren(elem.Children))
		return
	}
	if elem.SelfClose {
		if len(elem.Attributes) == 0 {
			p.line("<%s />", elem.Name)
			return
		}
		if len(p.renderOpenTag(elem)) <= 80 {
			p.line("%s />", p.renderOpenTag(elem))
			return
		}
		p.line("<%s", elem.Name)
		p.indent++
		for _, attr := range elem.Attributes {
			p.line(p.renderAttr(attr))
		}
		p.indent--
		p.line("/>")
		return
	}
	if isInlineChildren(elem.Children) {
		p.line("%s>%s</%s>", p.renderOpenTag(elem), p.inlineChildren(elem.Children), elem.Name)
		return
	}
	p.line("%s>", p.renderOpenTag(elem))
	p.indent++
	p.printNodes(elem.Children)
	p.indent--
	p.line("</%s>", elem.Name)
}

func (p *printer) inlineChildren(children []ast.Node) string {
	var out strings.Builder
	for _, child := range children {
		switch n := child.(type) {
		case *ast.Text:
			out.WriteString(n.Value)
		case *ast.Expr:
			out.WriteString("{" + strings.TrimSpace(n.Code) + "}")
		case *ast.Comment:
			out.WriteString("<!--" + strings.TrimSpace(n.Value) + "-->")
		case *ast.Element:
			inner := &printer{}
			inner.printElement(n)
			out.WriteString(strings.TrimSpace(inner.buf.String()))
		}
	}
	return out.String()
}

func isInlineChildren(children []ast.Node) bool {
	if len(children) == 0 {
		return true
	}
	length := 0
	for _, child := range children {
		switch n := child.(type) {
		case *ast.Text:
			if strings.Contains(n.Value, "\n") {
				return false
			}
			length += len(n.Value)
		case *ast.Expr:
			length += len(strings.TrimSpace(n.Code)) + 2
		case *ast.Comment:
			if strings.Contains(n.Value, "\n") {
				return false
			}
			length += len(strings.TrimSpace(n.Value)) + 7
		case *ast.Element:
			// Keep nested markup expanded for readability. Inline formatting is only
			// for text-like content, not for element trees.
			return false
		default:
			return false
		}
	}
	return length <= 80
}

func (p *printer) renderOpenTag(elem *ast.Element) string {
	if len(elem.Attributes) == 0 {
		return "<" + elem.Name
	}
	parts := []string{"<" + elem.Name}
	for _, attr := range elem.Attributes {
		parts = append(parts, p.renderAttr(attr))
	}
	return strings.Join(parts, " ")
}

func (p *printer) renderAttr(attr ast.Attribute) string {
	switch attr.Kind {
	case ast.AttrBool:
		return attr.Name
	case ast.AttrString:
		return fmt.Sprintf("%s=%q", attr.Name, attr.Value)
	default:
		return fmt.Sprintf("%s={%s}", attr.Name, strings.TrimSpace(attr.Expr))
	}
}

func (p *printer) line(format string, args ...any) {
	if format != "" {
		p.buf.WriteString(strings.Repeat("  ", p.indent))
		p.buf.WriteString(fmt.Sprintf(format, args...))
	}
	p.buf.WriteByte('\n')
}

func formatGoStatement(code string) string {
	src := "package gsxfmt\nfunc _() {\n" + code + "\n}\n"
	fset := token.NewFileSet()
	file, err := goparser.ParseFile(fset, "", src, 0)
	if err != nil || len(file.Decls) != 1 {
		return strings.TrimSpace(code)
	}
	fn, ok := file.Decls[0].(*goast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
		return strings.TrimSpace(code)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fn.Body.List[0]); err != nil {
		return strings.TrimSpace(code)
	}
	return strings.TrimSpace(buf.String())
}
