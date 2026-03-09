package compiler

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
)

type Registry struct {
	PackagesByDir        map[string]*Package
	PackagesByImportPath map[string]*Package
}

type Package struct {
	Dir        string
	Name       string
	ImportPath string
	Files      []*ast.File
	Components map[string]*Component
	Registry   *Registry
	TypeInfo   *TypeInfo
}

type Component struct {
	Decl      *ast.ComponentDecl
	File      *ast.File
	Params    []ast.Param
	ParamBy   map[string]ast.Param
	Slots     []string
	UsesRaw   bool
	HasLayout bool
}

type ResolvedComponent struct {
	Package     *Package
	Component   *Component
	ImportAlias string
	External    bool
}

func NewRegistry() *Registry {
	return &Registry{
		PackagesByDir:        map[string]*Package{},
		PackagesByImportPath: map[string]*Package{},
	}
}

func CompilePackage(dir string, files []*ast.File) (*Package, []diagnostics.Diagnostic) {
	pkg, diags := BuildPackage(dir, "", files)
	reg := NewRegistry()
	reg.Add(pkg)
	diags = append(diags, ValidateAll(reg)...)
	return pkg, diags
}

func BuildPackage(dir, importPath string, files []*ast.File) (*Package, []diagnostics.Diagnostic) {
	pkg := &Package{Dir: dir, ImportPath: importPath, Files: files, Components: map[string]*Component{}}
	var diags []diagnostics.Diagnostic
	for _, file := range files {
		if pkg.Name == "" {
			pkg.Name = file.Package
		}
		if file.Package != pkg.Name {
			diags = append(diags, diagnostics.Diagnostic{
				Severity: diagnostics.SeverityError,
				Code:     "C001",
				Message:  fmt.Sprintf("package %q does not match %q in %s", file.Package, pkg.Name, filepath.Base(file.Path)),
				Span:     diagnostics.NewSource(file.Path, file.Source).Span(0, 0),
			})
		}
		for _, decl := range file.Components {
			if prev, ok := pkg.Components[decl.Name]; ok {
				diags = append(diags, diagnostics.Diagnostic{
					Severity: diagnostics.SeverityError,
					Code:     "C002",
					Message:  fmt.Sprintf("duplicate component %s (previously declared in %s)", decl.Name, filepath.Base(prev.File.Path)),
					Span:     decl.Span,
				})
				continue
			}
			paramBy := map[string]ast.Param{}
			for _, param := range decl.Params {
				if _, ok := paramBy[param.Name]; ok {
					diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C003", Message: fmt.Sprintf("duplicate parameter %q on component %s", param.Name, decl.Name), Span: param.Span})
				}
				paramBy[param.Name] = param
			}
			slots, usesRaw := collectComponentMetadata(decl.Body)
			decl.Slots = slots
			decl.UsesRaw = usesRaw
			pkg.Components[decl.Name] = &Component{
				Decl:      decl,
				File:      file,
				Params:    decl.Params,
				ParamBy:   paramBy,
				Slots:     slots,
				UsesRaw:   usesRaw,
				HasLayout: len(slots) > 0,
			}
		}
	}
	analyzePackageTypes(pkg)
	return pkg, diags
}

func (r *Registry) Add(pkg *Package) {
	if pkg == nil {
		return
	}
	pkg.Registry = r
	r.PackagesByDir[pkg.Dir] = pkg
	if pkg.ImportPath != "" {
		r.PackagesByImportPath[pkg.ImportPath] = pkg
	}
}

func ValidateAll(reg *Registry) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, pkg := range reg.PackagesByDir {
		for _, comp := range pkg.Components {
			diags = append(diags, validateComponent(pkg, comp)...)
		}
	}
	return diags
}

func (pkg *Package) ResolveComponent(file *ast.File, name string) (*ResolvedComponent, bool) {
	if strings.Contains(name, ".") {
		alias, compName, ok := strings.Cut(name, ".")
		if !ok || alias == "" || compName == "" {
			return nil, false
		}
		importPath, ok := importPathForAlias(file, alias)
		if !ok || pkg.Registry == nil {
			return nil, false
		}
		otherPkg, ok := pkg.Registry.PackagesByImportPath[importPath]
		if !ok {
			return nil, false
		}
		comp, ok := otherPkg.Components[compName]
		if !ok {
			return nil, false
		}
		return &ResolvedComponent{Package: otherPkg, Component: comp, ImportAlias: alias, External: otherPkg != pkg}, true
	}
	comp, ok := pkg.Components[name]
	if !ok {
		return nil, false
	}
	return &ResolvedComponent{Package: pkg, Component: comp}, true
}

func collectComponentMetadata(nodes []ast.Node) ([]string, bool) {
	seen := map[string]bool{}
	var slots []string
	usesRaw := false
	var walk func([]ast.Node)
	walk = func(items []ast.Node) {
		for _, node := range items {
			switch n := node.(type) {
			case *ast.Element:
				if n.Name == "slot" {
					name := "default"
					if attr, ok := n.Attr("name"); ok && attr.Kind == ast.AttrString && attr.Value != "" {
						name = attr.Value
					}
					if !seen[name] {
						seen[name] = true
						slots = append(slots, name)
					}
				}
				if n.Name == "raw" {
					usesRaw = true
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
	return slots, usesRaw
}

func validateComponent(pkg *Package, comp *Component) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	var walkNodes func([]ast.Node)
	walkNodes = func(nodes []ast.Node) {
		for _, node := range nodes {
			switch n := node.(type) {
			case *ast.Element:
				diags = append(diags, validateElement(pkg, comp, n)...)
				walkNodes(n.Children)
			case *ast.If:
				for _, branch := range n.Branches {
					walkNodes(branch.Body)
				}
				walkNodes(n.ElseBody)
			case *ast.For:
				walkNodes(n.Body)
			}
		}
	}
	walkNodes(comp.Decl.Body)
	return diags
}

func validateElement(pkg *Package, comp *Component, elem *ast.Element) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	seenAttrs := map[string]ast.Attribute{}
	for _, attr := range elem.Attributes {
		if prev, ok := seenAttrs[attr.Name]; ok {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C004", Message: fmt.Sprintf("duplicate attribute %q", attr.Name), Span: attr.Span, Note: fmt.Sprintf("previous attribute at %s:%d:%d", prev.Span.File, prev.Span.Start.Line, prev.Span.Start.Column)})
		}
		seenAttrs[attr.Name] = attr
	}
	if isVoidElement(elem.Name) && len(elem.Children) > 0 {
		diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C005", Message: fmt.Sprintf("void element <%s> cannot have children", elem.Name), Span: elem.Span})
	}
	if elem.Name == "slot" {
		if len(elem.Children) > 0 {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C006", Message: "slot outlet cannot have children", Span: elem.Span})
		}
		if attr, ok := elem.Attr("name"); ok && attr.Kind != ast.AttrString {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C007", Message: "slot name must be a string literal", Span: attr.Span})
		}
		return diags
	}
	if elem.Name == "raw" {
		attr, ok := elem.Attr("html")
		if !ok || attr.Kind != ast.AttrExpression {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C008", Message: "raw node requires html={expr}", Span: elem.Span})
		}
		if len(elem.Children) > 0 {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C009", Message: "raw node cannot have children", Span: elem.Span})
		}
		return diags
	}
	if !elem.IsComponent() {
		return diags
	}
	resolved, diag := resolveElementComponent(pkg, comp, elem)
	if diag != nil {
		diags = append(diags, *diag)
		return diags
	}
	callee := resolved.Component
	attrNames := map[string]ast.Attribute{}
	for _, attr := range elem.Attributes {
		if attr.MetaSlot {
			continue
		}
		attrNames[attr.Name] = attr
		if _, ok := callee.ParamBy[attr.Name]; !ok {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C011", Message: fmt.Sprintf("unknown prop %q on component %s", attr.Name, callee.Decl.Name), Span: attr.Span})
		}
	}
	for _, param := range callee.Params {
		if _, ok := attrNames[param.Name]; !ok {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C012", Message: fmt.Sprintf("missing required prop %q on component %s", param.Name, callee.Decl.Name), Span: elem.Span})
		}
	}
	slotNames := map[string]bool{}
	for _, slot := range callee.Slots {
		slotNames[slot] = true
	}
	for _, child := range elem.Children {
		name := "default"
		if slotName, ok := childSlotName(child); ok {
			markMetaSlot(child)
			name = slotName
		}
		if len(slotNames) > 0 && !slotNames[name] {
			diags = append(diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C013", Message: fmt.Sprintf("component %s does not declare slot %q", callee.Decl.Name, name), Span: child.GetSpan()})
		}
	}
	return diags
}

func resolveElementComponent(pkg *Package, comp *Component, elem *ast.Element) (*ResolvedComponent, *diagnostics.Diagnostic) {
	if resolved, ok := pkg.ResolveComponent(comp.File, elem.Name); ok {
		return resolved, nil
	}
	if strings.Contains(elem.Name, ".") {
		alias, compName, ok := strings.Cut(elem.Name, ".")
		if !ok || alias == "" || compName == "" {
			diag := diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C010", Message: fmt.Sprintf("unknown component %s", elem.Name), Span: elem.Span}
			return nil, &diag
		}
		importPath, ok := importPathForAlias(comp.File, alias)
		if !ok {
			diag := diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C014", Message: fmt.Sprintf("unknown import alias %q for component %s", alias, elem.Name), Span: elem.Span}
			return nil, &diag
		}
		if pkg.Registry == nil || pkg.Registry.PackagesByImportPath[importPath] == nil {
			diag := diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C015", Message: fmt.Sprintf("imported GSX package %q was not discovered for component %s", importPath, elem.Name), Span: elem.Span}
			return nil, &diag
		}
		diag := diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C016", Message: fmt.Sprintf("component %s not found in imported package %q", compName, importPath), Span: elem.Span}
		return nil, &diag
	}
	diag := diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: "C010", Message: fmt.Sprintf("unknown component %s", elem.Name), Span: elem.Span}
	return nil, &diag
}

func importPathForAlias(file *ast.File, alias string) (string, bool) {
	for _, imp := range file.Imports {
		if importAlias(imp) == alias {
			return imp.Path, true
		}
	}
	return "", false
}

func importAlias(imp ast.ImportDecl) string {
	if imp.Alias != "" {
		return imp.Alias
	}
	return path.Base(imp.Path)
}

func markMetaSlot(node ast.Node) {
	elem, ok := node.(*ast.Element)
	if !ok {
		return
	}
	for i, attr := range elem.Attributes {
		if attr.Name == "slot" && attr.Kind == ast.AttrString {
			elem.Attributes[i].MetaSlot = true
		}
	}
}

func childSlotName(node ast.Node) (string, bool) {
	elem, ok := node.(*ast.Element)
	if !ok {
		return "", false
	}
	if elem.Name == "slot" {
		if attr, ok := elem.Attr("name"); ok && attr.Kind == ast.AttrString && attr.Value != "" {
			return attr.Value, true
		}
		return "default", true
	}
	for _, attr := range elem.Attributes {
		if attr.Name == "slot" && attr.Kind == ast.AttrString {
			return attr.Value, true
		}
	}
	return "", false
}

var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

func isVoidElement(name string) bool {
	return voidElements[name]
}

func SortedComponents(pkg *Package) []*Component {
	out := make([]*Component, 0, len(pkg.Components))
	for _, comp := range pkg.Components {
		out = append(out, comp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Decl.Name < out[j].Decl.Name })
	return out
}
