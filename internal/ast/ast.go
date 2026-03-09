package ast

import "github.com/jlcsoftgenie/gsx/internal/diagnostics"

type File struct {
	Package    string
	Imports    []ImportDecl
	Components []*ComponentDecl
	Path       string
	Source     string
}

type ImportDecl struct {
	Alias string
	Path  string
	Span  diagnostics.Span
}

type ComponentDecl struct {
	Name       string
	Params     []Param
	Body       []Node
	Span       diagnostics.Span
	Slots      []string
	UsesRaw    bool
	SourceFile string
}

type Param struct {
	Name string
	Type string
	Span diagnostics.Span
}

type Node interface {
	node()
	GetSpan() diagnostics.Span
}

type AttributeValueKind int

const (
	AttrString AttributeValueKind = iota
	AttrExpression
	AttrBool
)

type Attribute struct {
	Name     string
	Kind     AttributeValueKind
	Value    string
	Expr     string
	Span     diagnostics.Span
	MetaSlot bool
}

type Text struct {
	Value string
	Span  diagnostics.Span
}

func (*Text) node()                       {}
func (n *Text) GetSpan() diagnostics.Span { return n.Span }

type Expr struct {
	Code string
	Span diagnostics.Span
}

func (*Expr) node()                       {}
func (n *Expr) GetSpan() diagnostics.Span { return n.Span }

type Comment struct {
	Value string
	Span  diagnostics.Span
}

func (*Comment) node()                       {}
func (n *Comment) GetSpan() diagnostics.Span { return n.Span }

type Doctype struct {
	Value string
	Span  diagnostics.Span
}

func (*Doctype) node()                       {}
func (n *Doctype) GetSpan() diagnostics.Span { return n.Span }

type IfBranch struct {
	Cond string
	Body []Node
	Span diagnostics.Span
}

type If struct {
	Branches []IfBranch
	ElseBody []Node
	Span     diagnostics.Span
}

func (*If) node()                       {}
func (n *If) GetSpan() diagnostics.Span { return n.Span }

type For struct {
	Header string
	Body   []Node
	Span   diagnostics.Span
}

func (*For) node()                       {}
func (n *For) GetSpan() diagnostics.Span { return n.Span }

type Element struct {
	Name       string
	Attributes []Attribute
	Children   []Node
	SelfClose  bool
	Span       diagnostics.Span
}

func (*Element) node()                       {}
func (n *Element) GetSpan() diagnostics.Span { return n.Span }

func (n *Element) Attr(name string) (Attribute, bool) {
	for _, attr := range n.Attributes {
		if attr.Name == name {
			return attr, true
		}
	}
	return Attribute{}, false
}

func (n *Element) IsComponent() bool {
	if n.Name == "" {
		return false
	}
	r := n.Name[0]
	return (r >= 'A' && r <= 'Z') || containsDotWithCapitalizedTail(n.Name)
}

func containsDotWithCapitalizedTail(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '.' {
			r := s[i+1]
			return r >= 'A' && r <= 'Z'
		}
	}
	return false
}
