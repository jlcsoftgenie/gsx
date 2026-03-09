package linter

import (
	"fmt"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/compiler"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"github.com/jlcsoftgenie/gsx/internal/formatter"
)

func LintPackage(pkg *compiler.Package, checkFormat bool) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, file := range pkg.Files {
		if checkFormat {
			formatted, err := formatter.IsFormatted(file.Path, file.Source)
			if err == nil && !formatted {
				diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityWarning, Code: "L007", Message: "file is not formatted", Span: diagnostics.NewSource(file.Path, file.Source).Span(0, 0)})
			}
		}
		for _, decl := range file.Components {
			comp := pkg.Components[decl.Name]
			diags = append(diags, lintNodes(pkg, comp, decl.Body)...)
		}
	}
	return diags
}

func lintNodes(pkg *compiler.Package, comp *compiler.Component, nodes []ast.Node) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, node := range nodes {
		switch n := node.(type) {
		case *ast.Element:
			diags = append(diags, lintElement(pkg, comp, n)...)
			diags = append(diags, lintNodes(pkg, comp, n.Children)...)
		case *ast.If:
			for _, branch := range n.Branches {
				diags = append(diags, lintNodes(pkg, comp, branch.Body)...)
			}
			diags = append(diags, lintNodes(pkg, comp, n.ElseBody)...)
		case *ast.For:
			diags = append(diags, lintNodes(pkg, comp, n.Body)...)
		}
	}
	return diags
}

func lintElement(pkg *compiler.Package, comp *compiler.Component, elem *ast.Element) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	switch elem.Name {
	case "img":
		if _, ok := elem.Attr("alt"); !ok {
			diags = append(diags, warn("L001", "img element should include alt text", elem.Span))
		}
	case "button":
		if _, ok := elem.Attr("type"); !ok {
			diags = append(diags, warn("L002", "button element should include type", elem.Span))
		}
	case "raw":
		diags = append(diags, warn("L003", "raw HTML bypasses escaping; use only with trusted content", elem.Span))
	case "label":
		if _, ok := elem.Attr("for"); !ok && !containsDescendant(elem, "input") && !containsDescendant(elem, "select") && !containsDescendant(elem, "textarea") {
			diags = append(diags, warn("L004", "label should reference a form control or wrap one", elem.Span))
		}
	case "p":
		for _, child := range elem.Children {
			if childElem, ok := child.(*ast.Element); ok && isBlockElement(childElem.Name) {
				diags = append(diags, warn("L005", fmt.Sprintf("block element <%s> inside <p> is invalid HTML", childElem.Name), childElem.Span))
			}
		}
	}
	if elem.IsComponent() {
		if callee, ok := pkg.Components[elem.Name]; ok && len(callee.Slots) == 0 && len(elem.Children) > 0 {
			diags = append(diags, warn("L006", fmt.Sprintf("component %s does not render slots; child content is unused", callee.Decl.Name), elem.Span))
		}
	}
	for _, attr := range elem.Attributes {
		if strings.HasPrefix(attr.Name, "aria-") && attr.Kind == ast.AttrBool {
			diags = append(diags, warn("L008", fmt.Sprintf("ARIA attribute %q should have an explicit value", attr.Name), attr.Span))
		}
	}
	return diags
}

func warn(code, msg string, span diagnostics.Span) diagnostics.Diagnostic {
	return diagnostics.Diagnostic{Severity: diagnostics.SeverityWarning, Code: code, Message: msg, Span: span}
}

func containsDescendant(elem *ast.Element, name string) bool {
	for _, child := range elem.Children {
		childElem, ok := child.(*ast.Element)
		if !ok {
			continue
		}
		if childElem.Name == name || containsDescendant(childElem, name) {
			return true
		}
	}
	return false
}

func isBlockElement(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "div", "dl", "fieldset", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "main", "nav", "ol", "p", "pre", "section", "table", "ul":
		return true
	default:
		return false
	}
}
