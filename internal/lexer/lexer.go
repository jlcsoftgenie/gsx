package lexer

import (
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
)

type Kind int

const (
	Illegal Kind = iota
	EOF
	Ident
	String
	KeywordPackage
	KeywordImport
	KeywordComponent
	LParen
	RParen
	LBrace
	RBrace
	Comma
	Dot
)

type Token struct {
	Kind Kind
	Lit  string
	Span diagnostics.Span
}

type Lexer struct {
	src  *diagnostics.Source
	text string
	off  int
	peek *Token
}

func New(src *diagnostics.Source) *Lexer {
	return &Lexer{src: src, text: src.Text}
}

func (l *Lexer) Source() *diagnostics.Source { return l.src }
func (l *Lexer) Offset() int                 { return l.off }
func (l *Lexer) SetOffset(off int) {
	l.off = off
	l.peek = nil
}

func (l *Lexer) Peek() Token {
	if l.peek != nil {
		return *l.peek
	}
	tok := l.Next()
	l.peek = &tok
	return tok
}

func (l *Lexer) Next() Token {
	if l.peek != nil {
		tok := *l.peek
		l.peek = nil
		return tok
	}

	l.skipWhitespaceAndComments()
	start := l.off
	if start >= len(l.text) {
		return Token{Kind: EOF, Span: l.src.Span(start, start)}
	}

	r, width := utf8.DecodeRuneInString(l.text[start:])
	switch r {
	case '(':
		l.off += width
		return l.token(LParen, start, l.off)
	case ')':
		l.off += width
		return l.token(RParen, start, l.off)
	case '{':
		l.off += width
		return l.token(LBrace, start, l.off)
	case '}':
		l.off += width
		return l.token(RBrace, start, l.off)
	case ',':
		l.off += width
		return l.token(Comma, start, l.off)
	case '.':
		l.off += width
		return l.token(Dot, start, l.off)
	case '"', '`':
		return l.scanString()
	}

	if isIdentStart(r) {
		for l.off < len(l.text) {
			r, width := utf8.DecodeRuneInString(l.text[l.off:])
			if !isIdentContinue(r) {
				break
			}
			l.off += width
		}
		lit := l.text[start:l.off]
		switch lit {
		case "package":
			return Token{Kind: KeywordPackage, Lit: lit, Span: l.src.Span(start, l.off)}
		case "import":
			return Token{Kind: KeywordImport, Lit: lit, Span: l.src.Span(start, l.off)}
		case "component":
			return Token{Kind: KeywordComponent, Lit: lit, Span: l.src.Span(start, l.off)}
		default:
			return Token{Kind: Ident, Lit: lit, Span: l.src.Span(start, l.off)}
		}
	}

	l.off += width
	return Token{Kind: Illegal, Lit: string(r), Span: l.src.Span(start, l.off)}
}

func (l *Lexer) scanString() Token {
	quote := l.text[l.off]
	start := l.off
	l.off++
	for l.off < len(l.text) {
		c := l.text[l.off]
		l.off++
		if c == quote {
			break
		}
		if quote == '"' && c == '\\' && l.off < len(l.text) {
			l.off++
		}
	}
	return Token{Kind: String, Lit: l.text[start:l.off], Span: l.src.Span(start, l.off)}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for l.off < len(l.text) {
		if hasPrefix(l.text[l.off:], "//") {
			for l.off < len(l.text) && l.text[l.off] != '\n' {
				l.off++
			}
			continue
		}
		if hasPrefix(l.text[l.off:], "/*") {
			l.off += 2
			for l.off < len(l.text) && !hasPrefix(l.text[l.off:], "*/") {
				l.off++
			}
			if l.off < len(l.text) {
				l.off += 2
			}
			continue
		}
		r, width := utf8.DecodeRuneInString(l.text[l.off:])
		if !unicode.IsSpace(r) {
			break
		}
		l.off += width
	}
}

func (l *Lexer) Unquote(tok Token) (string, error) {
	return strconv.Unquote(tok.Lit)
}

func (l *Lexer) Slice(start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(l.text) {
		end = len(l.text)
	}
	if end < start {
		end = start
	}
	return l.text[start:end]
}

func (l *Lexer) token(kind Kind, start, end int) Token {
	return Token{Kind: kind, Lit: l.text[start:end], Span: l.src.Span(start, end)}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentContinue(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
