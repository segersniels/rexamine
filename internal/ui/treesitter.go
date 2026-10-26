package ui

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// treeSitterLanguages lists the languages that rexdiff colors with tree-sitter.
// Chroma keeps the languages whose grammars do worse on the parts of a file
// that a diff shows: Markdown and HTML need nested grammars for inline text
// and embedded scripts, PHP needs the "<?php" tag that most hunks leave out,
// and the SCSS query colors little. Release builds embed only these grammars:
// keep GRAMMAR_TAGS in the Makefile and .goreleaser.yaml in sync with this list.
var treeSitterLanguages = map[string]bool{
	"bash": true, "c": true, "c_sharp": true, "cpp": true, "css": true,
	"dart": true, "dockerfile": true, "elixir": true, "go": true, "gomod": true,
	"graphql": true, "hcl": true, "java": true, "javascript": true, "json": true,
	"kotlin": true, "lua": true, "make": true, "proto": true, "python": true,
	"ruby": true, "rust": true, "scala": true, "sql": true, "swift": true,
	"toml": true, "tsx": true, "typescript": true, "yaml": true, "zig": true,
}

// treeSitterHighlighters keeps one highlighter per language, because a new
// highlighter loads the grammar and compiles its query. A nil value marks a
// language whose query does not compile. Only the UI goroutine highlights, so
// the map needs no lock.
var treeSitterHighlighters = map[string]*gotreesitter.Highlighter{}

// treeSitterHighlighter returns the highlighter for the language of path, or
// nil when the language is not in treeSitterLanguages.
func treeSitterHighlighter(path string) *gotreesitter.Highlighter {
	entry := grammars.DetectLanguage(path)
	if entry == nil || !treeSitterLanguages[entry.Name] {
		return nil
	}
	if highlighter, ok := treeSitterHighlighters[entry.Name]; ok {
		return highlighter
	}

	query := entry.HighlightQuery
	if entry.Name == "javascript" || entry.Name == "tsx" {
		query += jsxHighlightQuery
	}

	// A query that does not compile gives no highlighter, which leaves the language to Chroma.
	highlighter, _ := gotreesitter.NewHighlighter(entry.Language(), query)
	treeSitterHighlighters[entry.Name] = highlighter
	return highlighter
}

// jsxHighlightQuery colors JSX tags and attributes. It copies highlights-jsx.scm
// from tree-sitter-javascript, which the bundled queries leave out. It comes
// last, so its "<" and ">" override the comparison operators.
const jsxHighlightQuery = `
(jsx_opening_element name: (identifier) @tag)
(jsx_closing_element name: (identifier) @tag)
(jsx_self_closing_element name: (identifier) @tag)

(jsx_attribute (property_identifier) @attribute)

(jsx_opening_element (["<" ">"]) @punctuation.bracket)
(jsx_closing_element (["</" ">"]) @punctuation.bracket)
(jsx_self_closing_element (["<" "/>"]) @punctuation.bracket)
`

// treeSitterSpans colors source with a tree-sitter highlighter.
func treeSitterSpans(highlighter *gotreesitter.Highlighter, palette syntaxPalette, source string) [][]span {
	var lines [][]span
	var line []span

	// appendText adds text to the current line, and starts a new line at each newline.
	appendText := func(text, color string) {
		for {
			before, after, found := strings.Cut(text, "\n")
			if before != "" {
				line = append(line, span{text: before, color: color})
			}
			if !found {
				return
			}
			lines = append(lines, line)
			line = nil
			text = after
		}
	}

	// The ranges are sorted and do not overlap, so the text between them is plain.
	offset := 0
	for _, r := range highlighter.Highlight([]byte(source)) {
		appendText(source[offset:r.StartByte], "")
		appendText(source[r.StartByte:r.EndByte], palette.captureColor(r.Capture))
		offset = int(r.EndByte)
	}
	appendText(source[offset:], "")
	return lines
}

// syntaxPalette holds GitHub's syntax colors as "#rrggbb" values.
type syntaxPalette struct {
	comment       string
	constant      string
	function      string
	keyword       string
	regexp        string
	stringLiteral string
	tag           string
	typeName      string
}

// darkPalette and lightPalette copy GitHub's Primer syntax colors.
var (
	darkPalette = syntaxPalette{
		comment:       "#8b949e",
		constant:      "#79c0ff",
		function:      "#d2a8ff",
		keyword:       "#ff7b72",
		regexp:        "#7ee787",
		stringLiteral: "#a5d6ff",
		tag:           "#7ee787",
		typeName:      "#ffa657",
	}
	lightPalette = syntaxPalette{
		comment:       "#59636e",
		constant:      "#0550ae",
		function:      "#6639ba",
		keyword:       "#cf222e",
		regexp:        "#116329",
		stringLiteral: "#0a3069",
		tag:           "#0550ae",
		typeName:      "#953800",
	}
)

// syntaxPaletteFor picks the palette that matches the markdown style.
func syntaxPaletteFor(markdownStyle string) syntaxPalette {
	if markdownStyle == "light" {
		return lightPalette
	}
	return darkPalette
}

// captureColor finds the color of a tree-sitter capture name, such as
// "function.method.builtin". It drops the last part of the name until a known
// name matches, so "function.method.builtin" falls back to "function". Names
// without a color, such as "variable" and "punctuation", stay plain.
func (p syntaxPalette) captureColor(capture string) string {
	for {
		switch capture {
		case "comment":
			return p.comment
		case "string", "character":
			return p.stringLiteral
		case "string.regex", "string.regexp", "string.special.regex":
			return p.regexp
		case "string.escape", "escape", "string.special.key", "string.special.symbol",
			"number", "float", "boolean", "constant", "variable.builtin", "label",
			"property", "variable.member", "field":
			return p.constant
		case "keyword", "conditional", "repeat", "include", "import", "exception",
			"storageclass", "return", "preproc", "operator", "type.builtin",
			"type.qualifier", "punctuation.special":
			return p.keyword
		case "function", "method", "attribute":
			return p.function
		case "type", "constructor":
			return p.typeName
		case "tag":
			return p.tag
		}

		i := strings.LastIndexByte(capture, '.')
		if i < 0 {
			return ""
		}
		capture = capture[:i]
	}
}
