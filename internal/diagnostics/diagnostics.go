package diagnostics

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Pos struct {
	Offset int
	Line   int
	Column int
}

type Span struct {
	File  string
	Start Pos
	End   Pos
}

func (s Span) IsZero() bool {
	return s.File == "" && s.Start.Offset == 0 && s.End.Offset == 0
}

type Diagnostic struct {
	Severity Severity
	Code     string
	Message  string
	Span     Span
	Note     string
}

type Source struct {
	Path        string
	Text        string
	lineOffsets []int
}

func NewSource(path, text string) *Source {
	offsets := []int{0}
	for i, r := range text {
		if r == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return &Source{Path: path, Text: text, lineOffsets: offsets}
}

func (s *Source) Position(offset int) Pos {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s.Text) {
		offset = len(s.Text)
	}
	idx := sort.Search(len(s.lineOffsets), func(i int) bool {
		return s.lineOffsets[i] > offset
	}) - 1
	if idx < 0 {
		idx = 0
	}
	lineStart := s.lineOffsets[idx]
	return Pos{Offset: offset, Line: idx + 1, Column: offset - lineStart + 1}
}

func (s *Source) Span(start, end int) Span {
	return Span{File: s.Path, Start: s.Position(start), End: s.Position(end)}
}

func Render(diag Diagnostic, src *Source) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s:%d:%d: %s", diag.Span.File, diag.Span.Start.Line, diag.Span.Start.Column, diag.Message)
	if diag.Code != "" {
		fmt.Fprintf(&buf, " [%s]", diag.Code)
	}
	if src == nil {
		if diag.Note != "" {
			fmt.Fprintf(&buf, "\n  note: %s", diag.Note)
		}
		return buf.String()
	}

	lines := strings.Split(src.Text, "\n")
	line := diag.Span.Start.Line
	if line > 0 && line <= len(lines) {
		text := lines[line-1]
		fmt.Fprintf(&buf, "\n%5d | %s", line, text)
		fmt.Fprintf(&buf, "\n      | %s^", strings.Repeat(" ", max(diag.Span.Start.Column-1, 0)))
	}
	if diag.Note != "" {
		fmt.Fprintf(&buf, "\n  note: %s", diag.Note)
	}
	return buf.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
