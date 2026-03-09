package compiler

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	"go/token"
	"go/types"
	"os"
	stdpath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gsxast "github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"golang.org/x/tools/go/packages"
)

type ValueKind uint8

const (
	ValueKindUnknown ValueKind = iota
	ValueKindString
	ValueKindBytes
	ValueKindBool
	ValueKindInt
	ValueKindUint
	ValueKindFloat
)

type TypeInfo struct {
	ExprKinds map[string]ValueKind
}

func (pkg *Package) ExprKind(span diagnostics.Span) ValueKind {
	if pkg == nil || pkg.TypeInfo == nil {
		return ValueKindUnknown
	}
	return pkg.TypeInfo.ExprKinds[spanKey(span)]
}

func spanKey(span diagnostics.Span) string {
	return strings.Join([]string{
		span.File,
		strconv.Itoa(span.Start.Offset),
		strconv.Itoa(span.End.Offset),
	}, ":")
}

func analyzePackageTypes(pkg *Package) {
	if pkg == nil || !canAnalyzeTypes(pkg) {
		return
	}
	src, exprSpans := buildTypecheckSource(pkg)
	if len(exprSpans) == 0 {
		return
	}
	overlayPath := filepath.Join(pkg.Dir, "zz_gsx_typecheck.go")
	if err := os.WriteFile(overlayPath, []byte(src), 0o644); err != nil {
		return
	}
	defer os.Remove(overlayPath)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles,
		Dir:  pkg.Dir,
	}
	loaded, err := packages.Load(cfg, ".")
	if err != nil || len(loaded) == 0 {
		return
	}
	p := loaded[0]
	if p.TypesInfo == nil {
		return
	}
	var overlayFile *goast.File
	for _, file := range p.Syntax {
		pos := p.Fset.Position(file.Package)
		if filepath.Clean(pos.Filename) == filepath.Clean(overlayPath) {
			overlayFile = file
			break
		}
	}
	if overlayFile == nil {
		return
	}
	exprKinds := map[string]ValueKind{}
	goast.Inspect(overlayFile, func(node goast.Node) bool {
		call, ok := node.(*goast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		ident, ok := call.Fun.(*goast.Ident)
		if !ok || !strings.HasPrefix(ident.Name, "__gsxMarkExpr") {
			return true
		}
		id, err := strconv.Atoi(strings.TrimPrefix(ident.Name, "__gsxMarkExpr"))
		if err != nil {
			return true
		}
		span, ok := exprSpans[id]
		if !ok {
			return true
		}
		kind := classifyType(p.TypesInfo.TypeOf(call.Args[0]))
		if kind != ValueKindUnknown {
			exprKinds[spanKey(span)] = kind
		}
		return true
	})
	if len(exprKinds) > 0 {
		pkg.TypeInfo = &TypeInfo{ExprKinds: exprKinds}
	}
}

func canAnalyzeTypes(pkg *Package) bool {
	entries, err := os.ReadDir(pkg.Dir)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := goparser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, goparser.PackageClauseOnly)
		if err != nil {
			return false
		}
		if file.Name != nil && file.Name.Name != pkg.Name {
			return false
		}
	}
	return true
}

type typecheckBuilder struct {
	pkg       *Package
	buf       strings.Builder
	indent    int
	exprID    int
	exprSpans map[int]diagnostics.Span
	imports   map[string]gsxast.ImportDecl
}

func buildTypecheckSource(pkg *Package) (string, map[int]diagnostics.Span) {
	b := &typecheckBuilder{
		pkg:       pkg,
		exprSpans: map[int]diagnostics.Span{},
		imports:   map[string]gsxast.ImportDecl{},
	}
	for _, file := range pkg.Files {
		for _, comp := range file.Components {
			b.collectImports(file, comp)
		}
	}
	b.line("package %s", pkg.Name)
	b.line("")
	if len(b.imports) > 0 {
		paths := make([]string, 0, len(b.imports))
		for importPath := range b.imports {
			paths = append(paths, importPath)
		}
		sort.Strings(paths)
		b.line("import (")
		b.indent++
		for _, importPath := range paths {
			imp := b.imports[importPath]
			alias := importAlias(imp)
			switch alias {
			case "", stdpath.Base(imp.Path):
				b.line("%q", imp.Path)
			default:
				b.line("%s %q", alias, imp.Path)
			}
		}
		b.indent--
		b.line(")")
		b.line("")
	}
	for _, file := range pkg.Files {
		for _, comp := range file.Components {
			b.emitComponent(comp)
			b.line("")
		}
	}
	for id := 1; id <= b.exprID; id++ {
		b.line("func __gsxMarkExpr%d[T any](v T) {}", id)
	}
	return b.buf.String(), b.exprSpans
}

func (b *typecheckBuilder) collectImports(file *gsxast.File, comp *gsxast.ComponentDecl) {
	for _, param := range comp.Params {
		b.collectImportsFromCode(file, param.Type)
	}
	b.collectImportsFromNodes(file, comp.Body)
}

func (b *typecheckBuilder) collectImportsFromNodes(file *gsxast.File, nodes []gsxast.Node) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *gsxast.Expr:
			b.collectImportsFromCode(file, n.Code)
		case *gsxast.If:
			for _, branch := range n.Branches {
				b.collectImportsFromCode(file, branch.Cond)
				b.collectImportsFromNodes(file, branch.Body)
			}
			b.collectImportsFromNodes(file, n.ElseBody)
		case *gsxast.For:
			b.collectImportsFromCode(file, n.Header)
			b.collectImportsFromNodes(file, n.Body)
		case *gsxast.Element:
			for _, attr := range n.Attributes {
				if attr.Kind == gsxast.AttrExpression {
					b.collectImportsFromCode(file, attr.Expr)
				}
			}
			b.collectImportsFromNodes(file, n.Children)
		}
	}
}

func (b *typecheckBuilder) collectImportsFromCode(file *gsxast.File, code string) {
	for _, imp := range file.Imports {
		alias := importAlias(imp)
		if alias == "" || alias == "_" || alias == "." {
			continue
		}
		if strings.Contains(code, alias+".") {
			b.imports[imp.Path] = imp
		}
	}
}

func (b *typecheckBuilder) emitComponent(comp *gsxast.ComponentDecl) {
	params := make([]string, 0, len(comp.Params))
	for _, param := range comp.Params {
		params = append(params, param.Name+" "+param.Type)
	}
	b.line("func __gsxTypecheck%s(%s) {", comp.Name, strings.Join(params, ", "))
	b.indent++
	b.emitNodes(comp.Body)
	b.indent--
	b.line("}")
}

func (b *typecheckBuilder) emitNodes(nodes []gsxast.Node) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *gsxast.Expr:
			b.markExpr(n.Code, n.Span)
		case *gsxast.If:
			if len(n.Branches) == 0 {
				continue
			}
			b.line("if %s {", n.Branches[0].Cond)
			b.indent++
			b.emitNodes(n.Branches[0].Body)
			b.indent--
			for _, branch := range n.Branches[1:] {
				b.line("} else if %s {", branch.Cond)
				b.indent++
				b.emitNodes(branch.Body)
				b.indent--
			}
			if len(n.ElseBody) > 0 {
				b.line("} else {")
				b.indent++
				b.emitNodes(n.ElseBody)
				b.indent--
			}
			b.line("}")
		case *gsxast.For:
			b.line("for %s {", n.Header)
			b.indent++
			b.emitNodes(n.Body)
			b.indent--
			b.line("}")
		case *gsxast.Element:
			for _, attr := range n.Attributes {
				if attr.Kind == gsxast.AttrExpression {
					b.markExpr(attr.Expr, attr.Span)
				}
			}
			b.emitNodes(n.Children)
		}
	}
}

func (b *typecheckBuilder) markExpr(code string, span diagnostics.Span) {
	b.exprID++
	id := b.exprID
	b.exprSpans[id] = span
	b.line("__gsxMarkExpr%d(%s)", id, code)
}

func (b *typecheckBuilder) line(format string, args ...any) {
	if format != "" {
		b.buf.WriteString(strings.Repeat("\t", b.indent))
		b.buf.WriteString(fmt.Sprintf(format, args...))
	}
	b.buf.WriteByte('\n')
}

func classifyType(t types.Type) ValueKind {
	if t == nil {
		return ValueKindUnknown
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		if u.Info()&types.IsBoolean != 0 {
			return ValueKindBool
		}
		if u.Info()&types.IsString != 0 {
			return ValueKindString
		}
		if u.Info()&types.IsFloat != 0 {
			return ValueKindFloat
		}
		if u.Info()&types.IsInteger != 0 {
			if isUnsignedBasic(u.Kind()) {
				return ValueKindUint
			}
			return ValueKindInt
		}
	case *types.Slice:
		if elem, ok := u.Elem().(*types.Basic); ok && elem.Kind() == types.Byte {
			return ValueKindBytes
		}
	}
	return ValueKindUnknown
}

func isUnsignedBasic(kind types.BasicKind) bool {
	switch kind {
	case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr:
		return true
	default:
		return false
	}
}
