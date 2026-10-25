package ui

import (
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/segersniels/rexdiff/internal/diff"
)

func TestHighlightHunkKeepsCommentStateAcrossLines(t *testing.T) {
	hunk := diff.Hunk{Lines: []diff.Line{
		{Kind: diff.Context, Text: "/* start of comment"},
		{Kind: diff.Removed, Text: "old comment line */"},
		{Kind: diff.Added, Text: "new comment line */"},
		{Kind: diff.Added, Text: "x := 1"},
	}}

	lines := highlightHunk(lexers.Match("main.go"), codeStyle("dark"), hunk)
	commentColor := lines[0][0].color
	if commentColor == "" {
		t.Fatal("comment line has no color")
	}
	for i := 1; i <= 2; i++ {
		if lines[i][0].color != commentColor {
			t.Errorf("line %d has color %q, want the comment color %q", i, lines[i][0].color, commentColor)
		}
	}
	if lines[3][0].color == commentColor {
		t.Error("code after the comment has the comment color")
	}
}

func TestHighlightLinesLeavesUnknownFilesPlain(t *testing.T) {
	lines := highlightLines(lexers.Match("notes.unknownext"), codeStyle("dark"), []string{"\tsome text"})
	if len(lines[0]) != 1 || lines[0][0] != (span{text: "    some text"}) {
		t.Errorf("got %+v, want one plain span with expanded tabs", lines[0])
	}
}

func TestHighlightLinesShowsControlCharactersAsSymbols(t *testing.T) {
	lines := []string{"s := \"x\ry\"", "b := 2\r", "c := \"\x1b[31m\""}
	want := []string{"s := \"x␍y\"", "b := 2", "c := \"␛[31m\""}

	got := highlightLines(lexers.Match("main.go"), codeStyle("dark"), lines)
	for i, spans := range got {
		text := ""
		for _, s := range spans {
			text += s.text
		}
		if text != want[i] {
			t.Errorf("line %d shows %q, want %q", i, text, want[i])
		}
	}
}

func TestSpansInCutsAcrossSpans(t *testing.T) {
	spans := []span{{text: "abc", color: "#111111"}, {text: "def", color: "#222222"}}

	got := spansIn(spans, 2, 5)
	want := []span{{text: "c", color: "#111111"}, {text: "de", color: "#222222"}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
