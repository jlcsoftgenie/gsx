package linter

import (
	"fmt"
	"slices"
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
			ctx := buildComponentLintContext(decl.Body)
			diags = append(diags, lintNodes(pkg, comp, decl.Body, ctx, false)...)
		}
	}
	return diags
}

type componentLintContext struct {
	ids       map[string]bool
	labelFors map[string]bool
}

func buildComponentLintContext(nodes []ast.Node) componentLintContext {
	ctx := componentLintContext{
		ids:       map[string]bool{},
		labelFors: map[string]bool{},
	}
	var walk func([]ast.Node)
	walk = func(items []ast.Node) {
		for _, node := range items {
			switch n := node.(type) {
			case *ast.Element:
				if attr, ok := n.Attr("id"); ok && attr.Kind == ast.AttrString && attr.Value != "" {
					ctx.ids[attr.Value] = true
				}
				if n.Name == "label" {
					if attr, ok := n.Attr("for"); ok && attr.Kind == ast.AttrString && attr.Value != "" {
						ctx.labelFors[attr.Value] = true
					}
				}
				walk(n.Children)
			case *ast.If:
				for _, branch := range n.Branches {
					walk(branch.Body)
				}
				walk(n.ElseBody)
			case *ast.For:
				walk(n.Body)
			}
		}
	}
	walk(nodes)
	return ctx
}

func lintNodes(pkg *compiler.Package, comp *compiler.Component, nodes []ast.Node, ctx componentLintContext, insideLabel bool) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, node := range nodes {
		switch n := node.(type) {
		case *ast.Element:
			diags = append(diags, lintElement(pkg, comp, n, ctx, insideLabel)...)
			diags = append(diags, lintNodes(pkg, comp, n.Children, ctx, insideLabel || n.Name == "label")...)
		case *ast.If:
			for _, branch := range n.Branches {
				diags = append(diags, lintNodes(pkg, comp, branch.Body, ctx, insideLabel)...)
			}
			diags = append(diags, lintNodes(pkg, comp, n.ElseBody, ctx, insideLabel)...)
		case *ast.For:
			diags = append(diags, lintNodes(pkg, comp, n.Body, ctx, insideLabel)...)
		}
	}
	return diags
}

func lintElement(pkg *compiler.Package, comp *compiler.Component, elem *ast.Element, ctx componentLintContext, insideLabel bool) []diagnostics.Diagnostic {
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
		if attr, ok := elem.Attr("for"); ok && attr.Kind == ast.AttrString && attr.Value != "" && !ctx.ids[attr.Value] {
			diags = append(diags, warn("L011", fmt.Sprintf("label for=%q does not match any id in this component", attr.Value), attr.Span))
		}
	case "p":
		for _, child := range elem.Children {
			if childElem, ok := child.(*ast.Element); ok && isBlockElement(childElem.Name) {
				diags = append(diags, warn("L005", fmt.Sprintf("block element <%s> inside <p> is invalid HTML", childElem.Name), childElem.Span))
			}
		}
	case "a":
		if attr, ok := elem.Attr("target"); ok && attr.Kind == ast.AttrString && attr.Value == "_blank" {
			if rel, ok := elem.Attr("rel"); !ok {
				diags = append(diags, warn("L009", `anchor with target="_blank" should include rel="noopener noreferrer"`, elem.Span))
			} else if rel.Kind == ast.AttrString {
				tokens := strings.Fields(strings.ToLower(rel.Value))
				if !slices.Contains(tokens, "noopener") || !slices.Contains(tokens, "noreferrer") {
					diags = append(diags, warn("L009", `anchor with target="_blank" should include rel="noopener noreferrer"`, rel.Span))
				}
			}
		}
	case "input", "select", "textarea":
		if !hasAccessibleControlLabel(elem, ctx, insideLabel) {
			diags = append(diags, warn("L010", fmt.Sprintf("%s element should have an associated label or aria-label", elem.Name), elem.Span))
		}
	}
	if elem.IsComponent() {
		if resolved, ok := pkg.ResolveComponent(comp.File, elem.Name); ok && len(resolved.Component.Slots) == 0 && len(elem.Children) > 0 {
			diags = append(diags, warn("L006", fmt.Sprintf("component %s does not render slots; child content is unused", resolved.Component.Decl.Name), elem.Span))
		}
	}
	for _, attr := range elem.Attributes {
		if strings.HasPrefix(attr.Name, "aria-") && attr.Kind == ast.AttrBool {
			diags = append(diags, warn("L008", fmt.Sprintf("ARIA attribute %q should have an explicit value", attr.Name), attr.Span))
		}
	}
	return diags
}

func hasAccessibleControlLabel(elem *ast.Element, ctx componentLintContext, insideLabel bool) bool {
	if insideLabel {
		return true
	}
	if hasAnyAttr(elem, "aria-label", "aria-labelledby") {
		return true
	}
	if elem.Name == "input" {
		if attr, ok := elem.Attr("type"); ok && attr.Kind == ast.AttrString {
			switch strings.ToLower(attr.Value) {
			case "hidden", "submit", "reset", "button", "image":
				return true
			}
		}
	}
	if attr, ok := elem.Attr("id"); ok && attr.Kind == ast.AttrString && attr.Value != "" {
		return ctx.labelFors[attr.Value]
	}
	return false
}

func hasAnyAttr(elem *ast.Element, names ...string) bool {
	for _, name := range names {
		if _, ok := elem.Attr(name); ok {
			return true
		}
	}
	return false
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
