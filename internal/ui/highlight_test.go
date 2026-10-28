package ui

import (
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/segersniels/rexamine/internal/diff"
)

func TestHighlightHunkKeepsCommentStateAcrossLines(t *testing.T) {
	hunk := diff.Hunk{Lines: []diff.Line{
		{Kind: diff.Context, Text: "/* start of comment"},
		{Kind: diff.Removed, Text: "old comment line */"},
		{Kind: diff.Added, Text: "new comment line */"},
		{Kind: diff.Added, Text: "x := 1"},
	}}

	lines := highlightHunk(fileColorer("main.go", "dark"), hunk)
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

func TestHighlightFilesReusesTheColorsOfUnchangedHunks(t *testing.T) {
	files := []diff.File{{Path: "main.go", Hunks: []diff.Hunk{
		{Lines: []diff.Line{{Kind: diff.Added, Text: "x := 1"}}},
		{Lines: []diff.Line{{Kind: diff.Added, Text: "y := 2"}}},
	}}}
	first, colors := highlightFiles(files, "dark", nil)

	files[0].Hunks[1] = diff.Hunk{Lines: []diff.Line{{Kind: diff.Added, Text: "// y := 2"}}}
	second, colors := highlightFiles(files, "dark", colors)

	if &second[0][0][0][0] != &first[0][0][0][0] {
		t.Error("the unchanged hunk was highlighted again")
	}
	if want := highlightHunk(fileColorer("main.go", "dark"), files[0].Hunks[1]); !reflect.DeepEqual(second[0][1], want) {
		t.Errorf("the changed hunk has colors %+v, want %+v", second[0][1], want)
	}
	if len(colors) != 2 {
		t.Errorf("the cache holds %d hunks, want only the 2 current hunks", len(colors))
	}
}

func TestHighlightLinesLeavesUnknownFilesPlain(t *testing.T) {
	lines := highlightLines(fileColorer("notes.unknownext", "dark"), []string{"\tsome text"})
	if len(lines[0]) != 1 || lines[0][0] != (span{text: "    some text"}) {
		t.Errorf("got %+v, want one plain span with expanded tabs", lines[0])
	}
}

func TestHighlightLinesShowsControlCharactersAsSymbols(t *testing.T) {
	lines := []string{"s := \"x\ry\"", "b := 2\r", "c := \"\x1b[31m\""}
	want := []string{"s := \"x␍y\"", "b := 2", "c := \"␛[31m\""}

	got := highlightLines(fileColorer("main.go", "dark"), lines)
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

func TestHighlightLinesColorsFunctionNames(t *testing.T) {
	lines := []string{"export async function getAccountAccess(id: string) {", "  await db.batch(id);", "}"}

	got := highlightLines(fileColorer("access.ts", "dark"), lines)
	for _, name := range []string{"getAccountAccess", "batch"} {
		if color := spanColor(got, name); color != darkPalette.function {
			t.Errorf("%s has color %q, want the function color %q", name, color, darkPalette.function)
		}
	}
}

func TestHighlightLinesColorsJSXTags(t *testing.T) {
	for _, path := range []string{"inbox.tsx", "inbox.jsx"} {
		got := highlightLines(fileColorer(path, "dark"), []string{`return <div className="list">{name}</div>;`})
		if color := spanColor(got, "div"); color != darkPalette.tag {
			t.Errorf("%s: div has color %q, want the tag color %q", path, color, darkPalette.tag)
		}
		if color := spanColor(got, "className"); color != darkPalette.function {
			t.Errorf("%s: className has color %q, want the attribute color %q", path, color, darkPalette.function)
		}
	}
}

func TestHighlightLinesFallsBackToChroma(t *testing.T) {
	got := highlightLines(fileColorer("build.bat", "dark"), []string{"REM build the app"})
	if spanColor(got, "REM build the app") == "" {
		t.Error("a language without a tree-sitter grammar has no colors")
	}
}

// spanColor returns the color of the span with exactly the given text.
func spanColor(lines [][]span, text string) string {
	for _, spans := range lines {
		for _, s := range spans {
			if s.text == text {
				return s.color
			}
		}
	}
	return ""
}

func TestReleaseBuildsEmbedEveryTreeSitterLanguage(t *testing.T) {
	for _, file := range []string{"../../Makefile", "../../.goreleaser.yaml"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for language := range treeSitterLanguages {
			tag := regexp.MustCompile(`\bgrammar_subset_` + language + `\b`)
			if !tag.Match(data) {
				t.Errorf("%s does not embed the %s grammar", file, language)
			}
		}
	}
}
