package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexdiff/internal/diff"
	"github.com/segersniels/rexdiff/internal/source"
)

// rowKind tells how a row is drawn and whether it can be commented on.
type rowKind int

const (
	proseRow rowKind = iota
	fileRow
	codeRow
)

// row is one screen line of the page. Code rows point back to the diff line
// they show, so a selection of rows maps to diff lines. A long diff line wraps
// over several code rows that point to the same line.
type row struct {
	kind rowKind

	// text is rendered. Code rows hold only the syntax colors of the line.
	text string

	// The fields below are set for file and code rows. file is -1 elsewhere.
	file   int
	hunk   int
	line   int
	gutter string
	change diff.Kind
}

// sameLine reports whether two code rows show the same diff line.
func (r row) sameLine(other row) bool {
	return r.kind == codeRow && other.kind == codeRow &&
		r.file == other.file && r.hunk == other.hunk && r.line == other.line
}

// layoutPage lays out the review as rows that are at most width wide.
// fileStarts holds the index of each file's first row. highlights holds the
// syntax colors of files. collapsed holds the paths of the files that show
// only their title.
func layoutPage(review source.Review, files []diff.File, highlights []highlightedFile, width int, markdownStyle string, collapsed map[string]bool) (rows []row, fileStarts []int) {
	addProse := func(text string) {
		for _, line := range strings.Split(text, "\n") {
			rows = append(rows, row{kind: proseRow, text: line, file: -1})
		}
	}

	markdown, err := newMarkdownRenderer(width, markdownStyle)
	if err != nil {
		addProse(errorStyle.Render(err.Error()))
		return rows, nil
	}

	addProse(renderHeader(review, files, width))
	addProse("")
	if pr := review.PullRequest; pr != nil {
		addProse(renderMarkdown(markdown, pr.Body, width))
		addProse("")
	}

	addProse(renderHeading(fmt.Sprintf("Files changed (%d)", len(files)), width))
	if len(files) == 0 {
		addProse("")
		addProse(dimStyle.Render("No changes."))
	}
	for i, file := range files {
		addProse("")
		fileStarts = append(fileStarts, len(rows))
		rows = append(rows, layoutFile(i, file, highlights[i], width, collapsed[file.Path])...)
	}
	return rows, fileStarts
}

// layoutFile lays out one file's title, hunk headers, and lines. A collapsed
// file shows only its title. highlighted holds the syntax colors of the lines.
func layoutFile(index int, file diff.File, highlighted highlightedFile, width int, collapsed bool) []row {
	var rows []row
	for _, line := range strings.Split(ansi.Wrap(fileTitle(file, collapsed), width, "/"), "\n") {
		rows = append(rows, row{kind: fileRow, text: line, file: index, hunk: -1})
	}
	if collapsed {
		return rows
	}

	if file.IsBinary {
		rows = append(rows, row{kind: fileRow, text: dimStyle.Render("Binary file not shown."), file: index, hunk: -1})
	}

	numberWidth := len(strconv.Itoa(maxLineNumber(file)))
	gutterWidth := numberWidth + len(" + ")
	textWidth := max(1, width-gutterWidth)

	for h, hunk := range file.Hunks {
		header := hunkStyle.Render(ansi.Truncate(hunk.Header, width, "…"))
		rows = append(rows, row{kind: fileRow, text: header, file: index, hunk: h})

		for l, line := range hunk.Lines {
			number := line.NewLine
			if line.Kind == diff.Removed {
				number = line.OldLine
			}

			text := displayText(line.Text)

			// Most lines fit, and a string is never wider than its length in bytes.
			chunks := []string{text}
			if len(text) > textWidth {
				chunks = strings.Split(ansi.Hardwrap(text, textWidth, true), "\n")
			}
			offset := 0
			for part, chunk := range chunks {
				colored := colorize(spansIn(highlighted[h][l], offset, offset+len(chunk)))
				offset += len(chunk)

				gutter := strings.Repeat(" ", gutterWidth)
				if part == 0 {
					gutter = fmt.Sprintf("%*d %s ", numberWidth, number, line.Kind.Sign())
				}
				rows = append(rows, row{
					kind: codeRow, text: colored, file: index, hunk: h, line: l,
					gutter: gutter, change: line.Kind,
				})
			}
		}
	}
	return rows
}

// fileTitle renders a chevron that shows whether the file is collapsed, the
// file's path, what happened to it, and its line counts.
func fileTitle(file diff.File, collapsed bool) string {
	chevron := "▾ "
	if collapsed {
		chevron = "▸ "
	}

	title := accentStyle.Render(chevron) + boldStyle.Render(file.Path)
	switch {
	case file.IsNew:
		title += dimStyle.Render(" (new)")
	case file.IsDeleted:
		title += dimStyle.Render(" (deleted)")
	case file.IsRenamed():
		title += dimStyle.Render(" ← " + file.OldPath)
	}

	added, removed := file.Stats()
	return title + " " + addedStyle.Render(fmt.Sprintf("+%d", added)) + " " + removedStyle.Render(fmt.Sprintf("−%d", removed))
}

// maxLineNumber finds the largest line number in the file, to size the gutter.
func maxLineNumber(file diff.File) int {
	largest := 1
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			largest = max(largest, line.OldLine, line.NewLine)
		}
	}
	return largest
}

// renderCode draws a code row at the full width, with the background of its
// change, or the selection background when it is selected.
func renderCode(r row, width int, selected bool) string {
	style := lipgloss.NewStyle()
	gutterStyle := dimStyle
	switch r.change {
	case diff.Added:
		style = style.Background(addedLineBg)
		gutterStyle = addedStyle.Background(addedLineBg)
	case diff.Removed:
		style = style.Background(removedLineBg)
		gutterStyle = removedStyle.Background(removedLineBg)
	}
	if selected {
		style = style.Background(selectedLineBg)
		gutterStyle = accentStyle.Background(selectedLineBg)
	}

	textWidth := max(0, width-ansi.StringWidth(r.gutter))
	return gutterStyle.Render(r.gutter) + style.Width(textWidth).Render(r.text)
}
