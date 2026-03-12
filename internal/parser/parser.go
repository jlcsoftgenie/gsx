package parser

import (
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jlcsoftgenie/gsx/internal/ast"
	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
	"github.com/jlcsoftgenie/gsx/internal/lexer"
)

type Parser struct {
	src   *diagnostics.Source
	lex   *lexer.Lexer
	diags []diagnostics.Diagnostic
}

func ParseFile(path, text string) (*ast.File, []diagnostics.Diagnostic) {
	src := diagnostics.NewSource(path, text)
	p := &Parser{src: src, lex: lexer.New(src)}
	file := &ast.File{Path: path, Source: text}

	if tok := p.lex.Next(); tok.Kind != lexer.KeywordPackage {
		p.error(tok.Span, "P001", "expected package declaration")
		return file, p.diags
	}
	pkgTok := p.lex.Next()
	if pkgTok.Kind != lexer.Ident {
		p.error(pkgTok.Span, "P002", "expected package name")
		return file, p.diags
	}
	file.Package = pkgTok.Lit

	for {
		tok := p.lex.Peek()
		switch tok.Kind {
		case lexer.KeywordImport:
			file.Imports = append(file.Imports, p.parseImportDecls()...)
		case lexer.KeywordComponent:
			decl := p.parseComponent()
			if decl != nil {
				decl.SourceFile = path
				file.Components = append(file.Components, decl)
			}
		case lexer.EOF:
			return file, p.diags
		default:
			p.error(tok.Span, "P003", fmt.Sprintf("unexpected token %q", tok.Lit))
			p.lex.Next()
		}
	}
}

func (p *Parser) parseImportDecls() []ast.ImportDecl {
	p.expect(lexer.KeywordImport, "expected import")
	if p.lex.Peek().Kind == lexer.LParen {
		p.lex.Next()
		var decls []ast.ImportDecl
		for p.lex.Peek().Kind != lexer.RParen && p.lex.Peek().Kind != lexer.EOF {
			decls = append(decls, p.parseImportSpec())
		}
		p.expect(lexer.RParen, "expected ) after import block")
		return decls
	}
	return []ast.ImportDecl{p.parseImportSpec()}
}

func (p *Parser) parseImportSpec() ast.ImportDecl {
	start := p.lex.Peek().Span
	alias := ""
	if tok := p.lex.Peek(); tok.Kind == lexer.Ident || tok.Kind == lexer.Dot {
		alias = p.lex.Next().Lit
	}
	strTok := p.lex.Next()
	if strTok.Kind != lexer.String {
		p.error(strTok.Span, "P004", "expected import path string")
		return ast.ImportDecl{Alias: alias, Span: start}
	}
	path, err := p.lex.Unquote(strTok)
	if err != nil {
		p.error(strTok.Span, "P005", "invalid import path string")
	}
	return ast.ImportDecl{Alias: alias, Path: path, Span: diagnostics.Span{File: p.src.Path, Start: start.Start, End: strTok.Span.End}}
}

func (p *Parser) parseComponent() *ast.ComponentDecl {
	startTok := p.expect(lexer.KeywordComponent, "expected component")
	nameTok := p.expect(lexer.Ident, "expected component name")
	if nameTok.Kind == lexer.EOF {
		return nil
	}
	p.expect(lexer.LParen, "expected ( after component name")
	params := p.parseParams()
	lbrace := p.expect(lexer.LBrace, "expected { after component signature")
	if lbrace.Kind == lexer.EOF {
		return nil
	}
	body, end := p.parseBlockNodes(p.lex.Offset())
	p.lex.SetOffset(end + 1)
	return &ast.ComponentDecl{
		Name:   nameTok.Lit,
		Params: params,
		Body:   body,
		Span:   diagnostics.Span{File: p.src.Path, Start: startTok.Span.Start, End: p.src.Position(end + 1)},
	}
}

func (p *Parser) parseParams() []ast.Param {
	var params []ast.Param
	for {
		if p.lex.Peek().Kind == lexer.RParen {
			p.lex.Next()
			return params
		}
		nameTok := p.expect(lexer.Ident, "expected parameter name")
		typeStart := p.lex.Offset()
		typeEnd, delim := p.scanUntilTopLevelDelimiter(typeStart, ',', ')')
		typeText := strings.TrimSpace(p.src.Text[typeStart:typeEnd])
		if typeText == "" {
			p.error(nameTok.Span, "P006", "expected parameter type")
		}
		params = append(params, ast.Param{
			Name: nameTok.Lit,
			Type: typeText,
			Span: diagnostics.Span{File: p.src.Path, Start: nameTok.Span.Start, End: p.src.Position(typeEnd)},
		})
		p.lex.SetOffset(typeEnd)
		tok := p.lex.Next()
		if tok.Kind == lexer.RParen || delim == ')' {
			return params
		}
		if tok.Kind != lexer.Comma {
			p.error(tok.Span, "P007", "expected , or ) after parameter")
			return params
		}
	}
}

func (p *Parser) parseBlockNodes(pos int) ([]ast.Node, int) {
	return p.parseNodeList(pos, "", true)
}

func (p *Parser) parseElementChildren(pos int, closingTag string) ([]ast.Node, int) {
	return p.parseNodeList(pos, closingTag, false)
}

func (p *Parser) parseNodeList(pos int, closingTag string, stopOnBrace bool) ([]ast.Node, int) {
	var nodes []ast.Node
	textStart := pos
	for pos < len(p.src.Text) {
		if stopOnBrace && p.src.Text[pos] == '}' {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			return nodes, pos
		}
		if closingTag != "" && p.matchClosingTag(pos, closingTag) {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			return nodes, pos
		}
		if hasPrefix(p.src.Text[pos:], "<!--") {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseComment(pos)
			nodes = append(nodes, node)
			pos = next
			textStart = pos
			continue
		}
		if hasPrefixFold(p.src.Text[pos:], "<!doctype") {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseDoctype(pos)
			nodes = append(nodes, node)
			pos = next
			textStart = pos
			continue
		}
		if p.src.Text[pos] == '<' {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseElement(pos)
			if node != nil {
				nodes = append(nodes, node)
			}
			if next <= pos {
				next = pos + 1
			}
			pos = next
			textStart = pos
			continue
		}
		if p.src.Text[pos] == '{' {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseExpr(pos)
			nodes = append(nodes, node)
			pos = next
			textStart = pos
			continue
		}
		if onlyWhitespace(p.src.Text[textStart:pos]) && keywordAt(p.src.Text, pos, "if") {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseIf(pos)
			nodes = append(nodes, node)
			pos = next
			textStart = pos
			continue
		}
		if onlyWhitespace(p.src.Text[textStart:pos]) && keywordAt(p.src.Text, pos, "for") {
			nodes = append(nodes, p.textNode(textStart, pos)...)
			node, next := p.parseFor(pos)
			nodes = append(nodes, node)
			pos = next
			textStart = pos
			continue
		}
		if onlyWhitespace(p.src.Text[textStart:pos]) {
			node, next, ok := p.parseDecl(pos)
			if ok {
				nodes = append(nodes, p.textNode(textStart, pos)...)
				if node != nil {
					nodes = append(nodes, node)
				}
				pos = next
				textStart = pos
				continue
			}
		}
		_, width := utf8.DecodeRuneInString(p.src.Text[pos:])
		pos += width
	}
	if stopOnBrace {
		p.error(p.src.Span(pos, pos), "P008", "expected } to close block")
	}
	nodes = append(nodes, p.textNode(textStart, pos)...)
	return nodes, pos
}

func (p *Parser) parseComment(pos int) (ast.Node, int) {
	end := strings.Index(p.src.Text[pos:], "-->")
	if end < 0 {
		p.error(p.src.Span(pos, pos), "P009", "unterminated comment")
		return &ast.Comment{Value: p.src.Text[pos+4:], Span: p.src.Span(pos, len(p.src.Text))}, len(p.src.Text)
	}
	endPos := pos + end + 3
	value := p.src.Text[pos+4 : pos+end]
	return &ast.Comment{Value: value, Span: p.src.Span(pos, endPos)}, endPos
}

func (p *Parser) parseDoctype(pos int) (ast.Node, int) {
	end := strings.IndexByte(p.src.Text[pos:], '>')
	if end < 0 {
		p.error(p.src.Span(pos, pos), "P010", "unterminated doctype")
		return &ast.Doctype{Value: strings.TrimSpace(p.src.Text[pos+2:]), Span: p.src.Span(pos, len(p.src.Text))}, len(p.src.Text)
	}
	endPos := pos + end + 1
	value := strings.TrimSpace(p.src.Text[pos+2 : pos+end])
	return &ast.Doctype{Value: value, Span: p.src.Span(pos, endPos)}, endPos
}

func (p *Parser) parseExpr(pos int) (ast.Node, int) {
	end := p.scanBalancedExpr(pos)
	if end < 0 {
		p.error(p.src.Span(pos, pos), "P011", "unterminated expression")
		return &ast.Expr{Code: strings.TrimSpace(p.src.Text[pos+1:]), Span: p.src.Span(pos, len(p.src.Text))}, len(p.src.Text)
	}
	code := strings.TrimSpace(p.src.Text[pos+1 : end])
	return &ast.Expr{Code: code, Span: p.src.Span(pos, end+1)}, end + 1
}

func (p *Parser) parseIf(pos int) (ast.Node, int) {
	headerStart := pos + len("if")
	headerEnd, blockStart := p.scanGoHeader(headerStart)
	cond := strings.TrimSpace(p.src.Text[headerStart:headerEnd])
	branches := []ast.IfBranch{}
	body, blockEnd := p.parseBlockNodes(blockStart + 1)
	branches = append(branches, ast.IfBranch{Cond: cond, Body: body, Span: p.src.Span(pos, blockEnd+1)})
	next := blockEnd + 1
	for {
		next = skipWhitespace(p.src.Text, next)
		if !keywordAt(p.src.Text, next, "else") {
			break
		}
		elseStart := next
		next += len("else")
		next = skipWhitespace(p.src.Text, next)
		if keywordAt(p.src.Text, next, "if") {
			headStart := next + len("if")
			headEnd, blkStart := p.scanGoHeader(headStart)
			branchBody, blkEnd := p.parseBlockNodes(blkStart + 1)
			branches = append(branches, ast.IfBranch{Cond: strings.TrimSpace(p.src.Text[headStart:headEnd]), Body: branchBody, Span: p.src.Span(elseStart, blkEnd+1)})
			next = blkEnd + 1
			continue
		}
		if next >= len(p.src.Text) || p.src.Text[next] != '{' {
			p.error(p.src.Span(next, next), "P012", "expected { after else")
			break
		}
		elseBody, elseEnd := p.parseBlockNodes(next + 1)
		return &ast.If{Branches: branches, ElseBody: elseBody, Span: p.src.Span(pos, elseEnd+1)}, elseEnd + 1
	}
	return &ast.If{Branches: branches, Span: p.src.Span(pos, next)}, next
}

func (p *Parser) parseFor(pos int) (ast.Node, int) {
	headerStart := pos + len("for")
	headerEnd, blockStart := p.scanGoHeader(headerStart)
	body, blockEnd := p.parseBlockNodes(blockStart + 1)
	return &ast.For{Header: strings.TrimSpace(p.src.Text[headerStart:headerEnd]), Body: body, Span: p.src.Span(pos, blockEnd+1)}, blockEnd + 1
}

func (p *Parser) parseDecl(pos int) (ast.Node, int, bool) {
	if !startsPotentialDecl(p.src.Text, pos) {
		return nil, pos, false
	}
	end, found := p.scanGoStatementEnd(pos)
	if !found {
		p.error(p.src.Span(pos, pos), "P024", "unterminated declaration statement")
		return nil, len(p.src.Text), true
	}
	code := strings.TrimSpace(p.src.Text[pos:end])
	if code == "" {
		return nil, pos, false
	}
	ok, err := isSupportedDeclStatement(code)
	if !ok {
		if err != nil {
			p.error(p.src.Span(pos, end), "P025", err.Error())
			return nil, end, true
		}
		return nil, pos, false
	}
	return &ast.Decl{Code: code, Span: p.src.Span(pos, end)}, end, true
}

func (p *Parser) parseElement(pos int) (ast.Node, int) {
	if hasPrefix(p.src.Text[pos:], "</") {
		p.error(p.src.Span(pos, pos+2), "P013", "unexpected closing tag")
		return nil, pos + 2
	}
	start := pos
	pos++
	nameStart := pos
	for pos < len(p.src.Text) && isTagNameChar(p.src.Text[pos]) {
		pos++
	}
	if nameStart == pos {
		p.error(p.src.Span(start, pos), "P014", "expected tag name")
		return nil, start + 1
	}
	name := p.src.Text[nameStart:pos]
	var attrs []ast.Attribute
	for {
		pos = skipWhitespace(p.src.Text, pos)
		if pos >= len(p.src.Text) {
			p.error(p.src.Span(start, pos), "P015", "unterminated start tag")
			return &ast.Element{Name: name, Attributes: attrs, Span: p.src.Span(start, pos)}, pos
		}
		if hasPrefix(p.src.Text[pos:], "/>") {
			return &ast.Element{Name: name, Attributes: attrs, SelfClose: true, Span: p.src.Span(start, pos+2)}, pos + 2
		}
		if p.src.Text[pos] == '>' {
			pos++
			children, childEnd := p.parseElementChildren(pos, name)
			end := p.consumeClosingTag(childEnd, name)
			return &ast.Element{Name: name, Attributes: attrs, Children: children, Span: p.src.Span(start, end)}, end
		}
		attr, next := p.parseAttribute(pos)
		attrs = append(attrs, attr)
		pos = next
	}
}

func (p *Parser) parseAttribute(pos int) (ast.Attribute, int) {
	start := pos
	for pos < len(p.src.Text) && isAttrNameChar(p.src.Text[pos]) {
		pos++
	}
	name := p.src.Text[start:pos]
	if name == "" {
		p.error(p.src.Span(start, start), "P016", "expected attribute name")
		return ast.Attribute{Span: p.src.Span(start, start)}, start + 1
	}
	pos = skipWhitespace(p.src.Text, pos)
	if pos >= len(p.src.Text) || p.src.Text[pos] != '=' {
		return ast.Attribute{Name: name, Kind: ast.AttrBool, Span: p.src.Span(start, pos)}, pos
	}
	pos++
	pos = skipWhitespace(p.src.Text, pos)
	if pos >= len(p.src.Text) {
		p.error(p.src.Span(start, pos), "P017", "expected attribute value")
		return ast.Attribute{Name: name, Kind: ast.AttrBool, Span: p.src.Span(start, pos)}, pos
	}
	switch p.src.Text[pos] {
	case '{':
		end := p.scanBalancedExpr(pos)
		if end < 0 {
			p.error(p.src.Span(start, pos), "P018", "unterminated attribute expression")
			return ast.Attribute{Name: name, Kind: ast.AttrExpression, Expr: strings.TrimSpace(p.src.Text[pos+1:]), Span: p.src.Span(start, len(p.src.Text))}, len(p.src.Text)
		}
		return ast.Attribute{Name: name, Kind: ast.AttrExpression, Expr: strings.TrimSpace(p.src.Text[pos+1 : end]), Span: p.src.Span(start, end+1)}, end + 1
	case '\'', '"':
		quote := p.src.Text[pos]
		valStart := pos + 1
		pos++
		for pos < len(p.src.Text) && p.src.Text[pos] != quote {
			pos++
		}
		if pos >= len(p.src.Text) {
			p.error(p.src.Span(start, pos), "P019", "unterminated attribute string")
			return ast.Attribute{Name: name, Kind: ast.AttrString, Value: p.src.Text[valStart:], Span: p.src.Span(start, len(p.src.Text))}, len(p.src.Text)
		}
		value := p.src.Text[valStart:pos]
		pos++
		return ast.Attribute{Name: name, Kind: ast.AttrString, Value: value, Span: p.src.Span(start, pos)}, pos
	default:
		p.error(p.src.Span(pos, pos+1), "P020", "expected quoted string or expression attribute value")
		return ast.Attribute{Name: name, Kind: ast.AttrBool, Span: p.src.Span(start, pos)}, pos
	}
}

func (p *Parser) consumeClosingTag(pos int, name string) int {
	if !p.matchClosingTag(pos, name) {
		p.error(p.src.Span(pos, pos), "P021", fmt.Sprintf("expected closing tag </%s>", name))
		return pos
	}
	pos += 2 + len(name)
	pos = skipWhitespace(p.src.Text, pos)
	if pos >= len(p.src.Text) || p.src.Text[pos] != '>' {
		p.error(p.src.Span(pos, pos), "P022", "expected > to close tag")
		return pos
	}
	return pos + 1
}

func (p *Parser) matchClosingTag(pos int, name string) bool {
	if !hasPrefix(p.src.Text[pos:], "</") {
		return false
	}
	pos += 2
	if !hasPrefix(p.src.Text[pos:], name) {
		return false
	}
	pos += len(name)
	if pos >= len(p.src.Text) {
		return false
	}
	if p.src.Text[pos] != '>' && !unicode.IsSpace(rune(p.src.Text[pos])) {
		return false
	}
	return true
}

func (p *Parser) scanBalancedExpr(pos int) int {
	depth := 0
	for i := pos + 1; i < len(p.src.Text); i++ {
		switch p.src.Text[i] {
		case '\'', '"':
			i = scanQuoted(p.src.Text, i)
		case '`':
			i = scanBacktick(p.src.Text, i)
		case '/':
			if hasPrefix(p.src.Text[i:], "//") {
				i = scanLineComment(p.src.Text, i)
			} else if hasPrefix(p.src.Text[i:], "/*") {
				i = scanBlockComment(p.src.Text, i)
			}
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

func (p *Parser) scanGoHeader(start int) (int, int) {
	round, square, brace := 0, 0, 0
	for i := start; i < len(p.src.Text); i++ {
		switch p.src.Text[i] {
		case '\'', '"':
			i = scanQuoted(p.src.Text, i)
		case '`':
			i = scanBacktick(p.src.Text, i)
		case '/':
			if hasPrefix(p.src.Text[i:], "//") {
				i = scanLineComment(p.src.Text, i)
			} else if hasPrefix(p.src.Text[i:], "/*") {
				i = scanBlockComment(p.src.Text, i)
			}
		case '(':
			round++
		case ')':
			if round > 0 {
				round--
			}
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{':
			if round == 0 && square == 0 && brace == 0 {
				return i, i
			}
			brace++
		case '}':
			if brace > 0 {
				brace--
			}
		}
	}
	p.error(p.src.Span(start, start), "P023", "expected { to start block")
	return len(p.src.Text), len(p.src.Text)
}

func (p *Parser) scanGoStatementEnd(start int) (int, bool) {
	round, square, brace := 0, 0, 0
	for i := start; i < len(p.src.Text); i++ {
		switch p.src.Text[i] {
		case '\'', '"':
			i = scanQuoted(p.src.Text, i)
		case '`':
			i = scanBacktick(p.src.Text, i)
		case '/':
			if hasPrefix(p.src.Text[i:], "//") {
				i = scanLineComment(p.src.Text, i)
				return i, true
			}
			if hasPrefix(p.src.Text[i:], "/*") {
				i = scanBlockComment(p.src.Text, i)
			}
		case '(':
			round++
		case ')':
			if round > 0 {
				round--
			}
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{':
			brace++
		case '}':
			if round == 0 && square == 0 && brace == 0 {
				return i, true
			}
			if brace > 0 {
				brace--
			}
		case '\n':
			if round == 0 && square == 0 && brace == 0 {
				return i, true
			}
		}
	}
	return len(p.src.Text), true
}

func (p *Parser) scanUntilTopLevelDelimiter(start int, delims ...byte) (int, byte) {
	round, square, brace := 0, 0, 0
	for i := start; i < len(p.src.Text); i++ {
		switch p.src.Text[i] {
		case '\'', '"':
			i = scanQuoted(p.src.Text, i)
		case '`':
			i = scanBacktick(p.src.Text, i)
		case '/':
			if hasPrefix(p.src.Text[i:], "//") {
				i = scanLineComment(p.src.Text, i)
			} else if hasPrefix(p.src.Text[i:], "/*") {
				i = scanBlockComment(p.src.Text, i)
			}
		case '(':
			round++
		case ')':
			if round == 0 && containsByte(delims, ')') {
				return i, ')'
			}
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			brace++
		case '}':
			brace--
		case ',':
			if round == 0 && square == 0 && brace == 0 && containsByte(delims, ',') {
				return i, ','
			}
		}
	}
	if containsByte(delims, ')') {
		return len(p.src.Text), ')'
	}
	return len(p.src.Text), 0
}

func (p *Parser) textNode(start, end int) []ast.Node {
	if end <= start {
		return nil
	}
	value := p.src.Text[start:end]
	if shouldDropText(value) {
		return nil
	}
	return []ast.Node{&ast.Text{Value: value, Span: p.src.Span(start, end)}}
}

func (p *Parser) error(span diagnostics.Span, code, msg string) {
	p.diags = append(p.diags, diagnostics.Diagnostic{Severity: diagnostics.SeverityError, Code: code, Message: msg, Span: span})
}

func (p *Parser) expect(kind lexer.Kind, msg string) lexer.Token {
	tok := p.lex.Next()
	if tok.Kind != kind {
		p.error(tok.Span, "P000", msg)
	}
	return tok
}

func shouldDropText(value string) bool {
	if value == "" {
		return true
	}
	return strings.ContainsRune(value, '\n') && strings.TrimSpace(value) == ""
}

func onlyWhitespace(value string) bool {
	return strings.TrimSpace(value) == ""
}

func startsPotentialDecl(src string, pos int) bool {
	if pos >= len(src) {
		return false
	}
	if keywordAt(src, pos, "var") || keywordAt(src, pos, "const") {
		return true
	}
	r, _ := utf8.DecodeRuneInString(src[pos:])
	if !isDeclIdentStart(r) {
		return false
	}
	end := pos
	for end < len(src) && src[end] != '\n' && src[end] != '}' {
		end++
	}
	segment := strings.TrimSpace(src[pos:end])
	return strings.Contains(segment, ":=") || strings.Contains(segment, "=") || strings.Contains(segment, "++") || strings.Contains(segment, "--")
}

func isDeclIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isSupportedDeclStatement(code string) (bool, error) {
	trimmed := strings.TrimSpace(code)
	src := "package gsxdecl\nfunc _() {\n" + trimmed + "\n}\n"
	file, err := goparser.ParseFile(gotoken.NewFileSet(), "", src, 0)
	if err != nil {
		if strings.HasPrefix(trimmed, "var") || strings.HasPrefix(trimmed, "const") || strings.Contains(trimmed, ":=") {
			return false, fmt.Errorf("invalid declaration statement")
		}
		return false, nil
	}
	if len(file.Decls) != 1 {
		return false, nil
	}
	fn, ok := file.Decls[0].(*goast.FuncDecl)
	if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
		if strings.HasPrefix(trimmed, "var") || strings.HasPrefix(trimmed, "const") || strings.Contains(trimmed, ":=") {
			return false, fmt.Errorf("declaration statement must be a single statement line")
		}
		return false, nil
	}
	switch stmt := fn.Body.List[0].(type) {
	case *goast.DeclStmt:
		decl, ok := stmt.Decl.(*goast.GenDecl)
		if !ok {
			return false, fmt.Errorf("unsupported declaration statement")
		}
		if decl.Tok == gotoken.VAR || decl.Tok == gotoken.CONST {
			return true, nil
		}
		return false, fmt.Errorf("only var and const declarations are supported in templates")
	case *goast.AssignStmt:
		if stmt.Tok == gotoken.DEFINE {
			return true, nil
		}
		return false, fmt.Errorf("reassignment is not supported in template bodies; use := to declare a new variable")
	case *goast.IncDecStmt:
		return false, fmt.Errorf("reassignment is not supported in template bodies; move mutation into Go code")
	case *goast.ExprStmt:
		return false, fmt.Errorf("standalone statements are not supported in template bodies")
	default:
		if strings.Contains(trimmed, ":=") {
			return false, fmt.Errorf("invalid declaration statement")
		}
		return false, fmt.Errorf("only local declarations, if blocks, and for blocks are supported in template bodies")
	}
}

func keywordAt(src string, pos int, keyword string) bool {
	if !hasPrefix(src[pos:], keyword) {
		return false
	}
	end := pos + len(keyword)
	if end < len(src) {
		r, _ := utf8.DecodeRuneInString(src[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			return false
		}
	}
	return true
}

func skipWhitespace(src string, pos int) int {
	for pos < len(src) {
		r, width := utf8.DecodeRuneInString(src[pos:])
		if !unicode.IsSpace(r) {
			break
		}
		pos += width
	}
	return pos
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return strings.EqualFold(s[:len(prefix)], prefix)
}

func containsByte(items []byte, want byte) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func isTagNameChar(ch byte) bool {
	return ch == '-' || ch == '_' || ch == ':' || ch == '.' ||
		(ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isAttrNameChar(ch byte) bool {
	return isTagNameChar(ch)
}

func scanQuoted(src string, pos int) int {
	quote := src[pos]
	for pos++; pos < len(src); pos++ {
		if src[pos] == '\\' && quote == '"' {
			pos++
			continue
		}
		if src[pos] == quote {
			return pos
		}
	}
	return len(src) - 1
}

func scanBacktick(src string, pos int) int {
	for pos++; pos < len(src); pos++ {
		if src[pos] == '`' {
			return pos
		}
	}
	return len(src) - 1
}

func scanLineComment(src string, pos int) int {
	for pos += 2; pos < len(src); pos++ {
		if src[pos] == '\n' {
			return pos
		}
	}
	return len(src) - 1
}

func scanBlockComment(src string, pos int) int {
	for pos += 2; pos < len(src)-1; pos++ {
		if src[pos] == '*' && src[pos+1] == '/' {
			return pos + 1
		}
	}
	return len(src) - 1
}
