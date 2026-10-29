package ui

import (
	"os"
	"reflect"
	"regexp"
	"slices"
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

	lines := highlightHunk(fileColorer("main.go", "dark", uiHighlighters), hunk)
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
	first, colors := highlightFiles(files, "dark", nil, nil)

	files[0].Hunks[1] = diff.Hunk{Lines: []diff.Line{{Kind: diff.Added, Text: "// y := 2"}}}
	second, colors := highlightFiles(files, "dark", colors, nil)

	if &second[0][0][0][0] != &first[0][0][0][0] {
		t.Error("the unchanged hunk was highlighted again")
	}
	if want := highlightHunk(fileColorer("main.go", "dark", uiHighlighters), files[0].Hunks[1]); !reflect.DeepEqual(second[0][1], want) {
		t.Errorf("the changed hunk has colors %+v, want %+v", second[0][1], want)
	}
	if len(colors) != 2 {
		t.Errorf("the cache holds %d hunks, want only the 2 current hunks", len(colors))
	}
}

func TestHighlightLinesLeavesUnknownFilesPlain(t *testing.T) {
	lines := highlightLines(fileColorer("notes.unknownext", "dark", uiHighlighters), []string{"\tsome text"})
	if len(lines[0]) != 1 || lines[0][0] != (span{text: "    some text"}) {
		t.Errorf("got %+v, want one plain span with expanded tabs", lines[0])
	}
}

func TestHighlightLinesShowsControlCharactersAsSymbols(t *testing.T) {
	lines := []string{"s := \"x\ry\"", "b := 2\r", "c := \"\x1b[31m\""}
	want := []string{"s := \"x␍y\"", "b := 2", "c := \"␛[31m\""}

	got := highlightLines(fileColorer("main.go", "dark", uiHighlighters), lines)
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

	got := highlightLines(fileColorer("access.ts", "dark", uiHighlighters), lines)
	for _, name := range []string{"getAccountAccess", "batch"} {
		if color := spanColor(got, name); color != darkPalette.function {
			t.Errorf("%s has color %q, want the function color %q", name, color, darkPalette.function)
		}
	}
}

func TestHighlightLinesColorsJSXTags(t *testing.T) {
	for _, path := range []string{"inbox.tsx", "inbox.jsx"} {
		got := highlightLines(fileColorer(path, "dark", uiHighlighters), []string{`return <div className="list">{name}</div>;`})
		if color := spanColor(got, "div"); color != darkPalette.tag {
			t.Errorf("%s: div has color %q, want the tag color %q", path, color, darkPalette.tag)
		}
		if color := spanColor(got, "className"); color != darkPalette.function {
			t.Errorf("%s: className has color %q, want the attribute color %q", path, color, darkPalette.function)
		}
	}
}

func TestHighlightLinesFallsBackToChroma(t *testing.T) {
	got := highlightLines(fileColorer("build.bat", "dark", uiHighlighters), []string{"REM build the app"})
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

// fakeFiles serves fixed file texts as a source.FileReader.
type fakeFiles struct {
	oldTexts, newTexts map[string]string
}

func (f fakeFiles) ReadFiles(_, _ []string) (map[string]string, map[string]string) {
	return f.oldTexts, f.newTexts
}

func TestWholeFileColorsFixAHunkThatStartsAtAClosingJSXTag(t *testing.T) {
	file := "const Page = () => (\n  <Button.Wrapper>\n    ok\n  </Button.Wrapper>\n);\n\nconst renderNotice = () => {\n  return null;\n};\n"
	parsed := diff.Parse("diff --git a/page.tsx b/page.tsx\n--- a/page.tsx\n+++ b/page.tsx\n" +
		"@@ -4,3 +4,7 @@\n   </Button.Wrapper>\n );\n \n+const renderNotice = () => {\n+  return null;\n+};\n+\n")
	keyword := darkPalette.keyword

	alone, _ := highlightFiles(parsed, "dark", nil, nil)
	if got := alone[0][0][3][0]; got.text == "const" && got.color == keyword {
		t.Fatal("the hunk alone colors const already, so the test does not cover the parser's error")
	}

	msg := colorWholeFiles(1, fakeFiles{newTexts: map[string]string{"page.tsx": file}}, parsed, "dark")().(wholeFilesMsg)
	whole, _ := highlightFiles(parsed, "dark", nil, msg.colors)
	if got := whole[0][0][3][0]; got.text != "const" || got.color != keyword {
		t.Errorf("const has span %+v, want the keyword color %s", got, keyword)
	}
}

func TestWholeFileColorsSkipLinesThatChangedInTheFile(t *testing.T) {
	hunk := diff.Hunk{Lines: []diff.Line{{Kind: diff.Added, Text: "x := 1", NewLine: 1}}}
	hunkLines := [][]span{{{text: "x := 1", color: "#hunk"}}}
	colors := wholeFileColors{newVersion: versionColors{
		lines: []string{"y := 2"},
		spans: [][]span{{{text: "y := 2", color: "#file"}}},
	}}

	if got := colors.apply(hunk, hunkLines); !reflect.DeepEqual(got, hunkLines) {
		t.Errorf("got %+v, want the hunk colors %+v", got, hunkLines)
	}
}

func TestAnOlderWholeFileJobSkipsItsWork(t *testing.T) {
	older := colorWholeFiles(1, fakeFiles{}, nil, "dark")
	colorWholeFiles(2, fakeFiles{}, nil, "dark")

	if msg := older(); msg != nil {
		t.Errorf("the older job returned %+v, want no message", msg)
	}
}

func TestWrapWordsBreaksAtSpaces(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  []string
	}{
		{"one two three", 8, []string{"one two", "three"}},
		{"  const value = 1", 12, []string{"  const", "value = 1"}},
		{"createGmailClient(id)", 8, []string{"createGm", "ailClien", "t(id)"}},
	}
	for _, tt := range tests {
		var got []string
		for _, line := range wrapWords(tt.text, tt.width) {
			got = append(got, tt.text[line[0]:line[1]])
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("wrapWords(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
		}
	}
}
