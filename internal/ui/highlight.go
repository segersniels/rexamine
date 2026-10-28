package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexamine/internal/diff"
)

// span is a part of a code line with the color of its syntax. color is a
// "#rrggbb" value, or empty for the terminal's default text color.
type span struct {
	text  string
	color string
}

// highlightedFile holds the syntax colors of a file's lines, by hunk and line.
type highlightedFile [][][]span

// hunkColors holds the syntax colors of hunks' lines, by hunkKey.
type hunkColors map[string][][]span

// highlightFiles colors the lines of files for the language that each file
// name implies. markdownStyle picks light or dark colors. It reuses the colors
// in previous for the hunks that did not change, because a parse is slow and
// a reload while the agent works changes few hunks. It also returns the
// colors of only these files' hunks, for the next call.
func highlightFiles(files []diff.File, markdownStyle string, previous hunkColors) ([]highlightedFile, hunkColors) {
	result := make([]highlightedFile, len(files))
	current := hunkColors{}
	for i, file := range files {
		colorSource := fileColorer(file.Path, markdownStyle)
		for _, hunk := range file.Hunks {
			key := hunkKey(file.Path, hunk)
			lines, ok := previous[key]
			if !ok {
				lines = highlightHunk(colorSource, hunk)
			}
			current[key] = lines
			result[i] = append(result[i], lines)
		}
	}
	return result, current
}

// hunkKey identifies a hunk by everything that its colors depend on: the
// file path, which picks the language, and the kind and text of each line.
func hunkKey(path string, hunk diff.Hunk) string {
	var b strings.Builder
	b.WriteString(path)
	for _, line := range hunk.Lines {
		b.WriteByte(0)
		b.WriteString(line.Kind.Sign())
		b.WriteString(line.Text)
	}
	return b.String()
}

// sourceColorer colors the lines of a source text that ends with a newline,
// and returns the spans of each line.
type sourceColorer func(source string) [][]span

// fileColorer picks how to color a file. A tree-sitter grammar parses the
// code, so it can tell function names, types, and properties apart. Chroma
// covers the languages that rexamine has no grammar for. The result is nil
// when neither knows the language.
func fileColorer(path, markdownStyle string) sourceColorer {
	if highlighter := treeSitterHighlighter(path); highlighter != nil {
		palette := syntaxPaletteFor(markdownStyle)
		return func(source string) [][]span {
			return treeSitterSpans(highlighter, palette, source)
		}
	}

	if lexer := lexers.Match(path); lexer != nil {
		style := codeStyle(markdownStyle)
		return func(source string) [][]span {
			return chromaSpans(lexer, style, source)
		}
	}
	return nil
}

// codeStyle picks the Chroma colors that match the markdown style. Both follow
// GitHub, like the diff colors.
func codeStyle(markdownStyle string) *chroma.Style {
	if markdownStyle == "light" {
		return styles.Get("github")
	}
	return styles.Get("github-dark")
}

// highlightHunk colors each line of a hunk. It highlights the old and the new
// side of the hunk as separate texts, so comments and strings that span
// several lines get the correct colors. A nil colorSource leaves the lines
// plain.
func highlightHunk(colorSource sourceColorer, hunk diff.Hunk) [][]span {
	var oldLines, newLines []string
	for _, line := range hunk.Lines {
		if line.Kind != diff.Added {
			oldLines = append(oldLines, line.Text)
		}
		if line.Kind != diff.Removed {
			newLines = append(newLines, line.Text)
		}
	}
	oldSpans := highlightLines(colorSource, oldLines)
	newSpans := highlightLines(colorSource, newLines)

	result := make([][]span, len(hunk.Lines))
	oldIndex, newIndex := 0, 0
	for i, line := range hunk.Lines {
		switch line.Kind {
		case diff.Removed:
			result[i] = oldSpans[oldIndex]
			oldIndex++
		case diff.Added:
			result[i] = newSpans[newIndex]
			newIndex++
		default:
			result[i] = newSpans[newIndex]
			oldIndex++
			newIndex++
		}
	}
	return result
}

// highlightLines colors consecutive lines of one file. It colors the lines as
// displayText shows them, so the spans make up the same text as the rest of
// the diff view.
func highlightLines(colorSource sourceColorer, lines []string) [][]span {
	shown := make([]string, len(lines))
	result := make([][]span, len(lines))
	for i, line := range lines {
		shown[i] = displayText(line)
		result[i] = []span{{text: shown[i]}}
	}
	if colorSource == nil || len(lines) == 0 {
		return result
	}

	for i, spans := range colorSource(strings.Join(shown, "\n") + "\n") {
		if i >= len(lines) {
			break
		}
		result[i] = spans
	}
	return result
}

// chromaSpans colors source with a Chroma lexer.
func chromaSpans(lexer chroma.Lexer, style *chroma.Style, source string) [][]span {
	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, source)
	if err != nil {
		return nil
	}

	// The style's base text color is left out, so plain code uses the terminal's own color.
	baseColor := style.Get(chroma.Background).Colour
	var result [][]span
	for _, tokens := range chroma.SplitTokensIntoLines(iterator.Tokens()) {
		var spans []span
		for _, token := range tokens {
			text := strings.TrimSuffix(token.Value, "\n")
			if text == "" {
				continue
			}

			color := ""
			if colour := style.Get(token.Type).Colour; colour.IsSet() && colour != baseColor {
				color = colour.String()
			}
			spans = append(spans, span{text: text, color: color})
		}
		result = append(result, spans)
	}
	return result
}

// displayText prepares a code line for the terminal. It expands each tab to
// four spaces and drops the carriage return of a Windows line ending. It shows
// other control characters as symbols, such as ␍, because the terminal would
// act on them: a carriage return moves the cursor and an escape changes colors.
func displayText(line string) string {
	line = strings.ReplaceAll(strings.TrimSuffix(line, "\r"), "\t", "    ")
	return strings.Map(func(r rune) rune {
		switch {
		case r < 0x20:
			return 0x2400 + r // the Control Pictures block, such as ␍ for \r
		case r == 0x7f:
			return '␡'
		default:
			return r
		}
	}, line)
}

// spansIn cuts the parts of spans that lie between the byte offsets start and
// end of the line that the spans make up.
func spansIn(spans []span, start, end int) []span {
	var result []span
	offset := 0
	for _, s := range spans {
		from, to := max(start, offset), min(end, offset+len(s.text))
		if from < to {
			result = append(result, span{text: s.text[from-offset : to-offset], color: s.color})
		}
		offset += len(s.text)
	}
	return result
}

// colorize renders spans in their colors. Each span resets only the text
// color, so the background of the row stays intact. Bubble Tea lowers the
// colors to what the terminal supports.
func colorize(spans []span) string {
	var b strings.Builder
	for _, s := range spans {
		if s.color == "" {
			b.WriteString(s.text)
			continue
		}
		color := ansi.NewStyle().ForegroundColor(lipgloss.Color(s.color)).String()
		b.WriteString(color + s.text + ansi.NewStyle().ForegroundColor(nil).String())
	}
	return b.String()
}
