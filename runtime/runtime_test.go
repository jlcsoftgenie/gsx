package runtime

import (
	"strings"
	"testing"
)

func TestWriteEscaped(t *testing.T) {
	var b strings.Builder
	if err := WriteEscaped(&b, `<script>alert("x")</script>`); err != nil {
		t.Fatal(err)
	}
	want := "&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;"
	if b.String() != want {
		t.Fatalf("got %q want %q", b.String(), want)
	}
}

func TestWriteRaw(t *testing.T) {
	var b strings.Builder
	if err := WriteRaw(&b, HTML("<strong>ok</strong>")); err != nil {
		t.Fatal(err)
	}
	if b.String() != "<strong>ok</strong>" {
		t.Fatalf("got %q", b.String())
	}
}

func TestWriteBooleanAttr(t *testing.T) {
	var b strings.Builder
	if err := WriteAttr(&b, "disabled", true, true); err != nil {
		t.Fatal(err)
	}
	if b.String() != " disabled" {
		t.Fatalf("got %q", b.String())
	}
}

func TestWriteTypedAttr(t *testing.T) {
	var b strings.Builder
	if err := WriteAttrInt64(&b, "data-count", 42); err != nil {
		t.Fatal(err)
	}
	if b.String() != ` data-count="42"` {
		t.Fatalf("got %q", b.String())
	}
}

func TestWriteTypedEscapedString(t *testing.T) {
	var b strings.Builder
	if err := WriteEscapedString(&b, `<b>hi</b>`); err != nil {
		t.Fatal(err)
	}
	if b.String() != "&lt;b&gt;hi&lt;/b&gt;" {
		t.Fatalf("got %q", b.String())
	}
}
