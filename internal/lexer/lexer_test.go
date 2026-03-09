package lexer

import (
	"testing"

	"github.com/jlcsoftgenie/gsx/internal/diagnostics"
)

func TestLexerTopLevelTokens(t *testing.T) {
	src := diagnostics.NewSource("test.gsx", `package pages
import "fmt"
component Card(title string) { <div>{title}</div> }
`)
	lex := New(src)
	kinds := []Kind{
		KeywordPackage, Ident,
		KeywordImport, String,
		KeywordComponent, Ident, LParen, Ident, Ident, RParen, LBrace,
	}
	for i, want := range kinds {
		got := lex.Next()
		if got.Kind != want {
			t.Fatalf("token %d: got %v want %v (%q)", i, got.Kind, want, got.Lit)
		}
	}
}
