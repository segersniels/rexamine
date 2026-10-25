package ui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/segersniels/rexdiff/internal/diff"
)

// span is a part of a code line with the color of its syntax. color is a
// "#rrggbb" value, or empty for the terminal's default text color.
type span struct {
	text  string
	color string
}

// highlightedFile holds the syntax colors of a file's lines, by hunk and line.
type highlightedFile [][][]span

// highlightFiles colors the lines of files for the language that each file
// name implies. markdownStyle picks light or dark colors.
func highlightFiles(files []diff.File, markdownStyle string) []highlightedFile {
	style := codeStyle(markdownStyle)

	result := make([]highlightedFile, len(files))
	for i, file := range files {
		lexer := lexers.Match(file.Path)
		for _, hunk := range file.Hunks {
			result[i] = append(result[i], highlightHunk(lexer, style, hunk))
		}
	}
	return result
}

// codeStyle picks the syntax colors that match the markdown style. Both follow
// GitHub, like the diff colors.
func codeStyle(markdownStyle string) *chroma.Style {
	if markdownStyle == "light" {
		return styles.Get("github")
	}
	return styles.Get("github-dark")
}

// highlightHunk colors each line of a hunk. It highlights the old and the new
// side of the hunk as separate texts, so comments and strings that span
// several lines get the correct colors. A nil lexer leaves the lines plain.
func highlightHunk(lexer chroma.Lexer, style *chroma.Style, hunk diff.Hunk) [][]span {
	var oldLines, newLines []string
	for _, line := range hunk.Lines {
		if line.Kind != diff.Added {
			oldLines = append(oldLines, line.Text)
		}
		if line.Kind != diff.Removed {
			newLines = append(newLines, line.Text)
		}
	}
	oldSpans := highlightLines(lexer, style, oldLines)
	newSpans := highlightLines(lexer, style, newLines)

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
func highlightLines(lexer chroma.Lexer, style *chroma.Style, lines []string) [][]span {
	shown := make([]string, len(lines))
	result := make([][]span, len(lines))
	for i, line := range lines {
		shown[i] = displayText(line)
		result[i] = []span{{text: shown[i]}}
	}
	if lexer == nil || len(lines) == 0 {
		return result
	}

	iterator, err := chroma.Coalesce(lexer).Tokenise(nil, strings.Join(shown, "\n")+"\n")
	if err != nil {
		return result
	}

	// The style's base text color is left out, so plain code uses the terminal's own color.
	baseColor := style.Get(chroma.Background).Colour
	for i, tokens := range chroma.SplitTokensIntoLines(iterator.Tokens()) {
		if i >= len(lines) {
			break
		}

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
		result[i] = spans
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
// color, so the background of the row stays intact.
func colorize(spans []span) string {
	profile := lipgloss.ColorProfile()

	var b strings.Builder
	for _, s := range spans {
		sequence := ""
		if color := profile.Color(s.color); color != nil {
			sequence = color.Sequence(false)
		}
		if sequence == "" {
			b.WriteString(s.text)
			continue
		}
		b.WriteString(termenv.CSI + sequence + "m" + s.text + termenv.CSI + "39m")
	}
	return b.String()
}
