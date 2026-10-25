// Package diff parses the unified diffs that git and gh print.
package diff

import (
	"strconv"
	"strings"
)

// Kind tells whether a diff line was kept, added, or removed.
type Kind int

// The kinds of diff line.
const (
	Context Kind = iota
	Added
	Removed
)

// Sign returns the character that marks the kind in a unified diff.
func (k Kind) Sign() string {
	switch k {
	case Added:
		return "+"
	case Removed:
		return "-"
	default:
		return " "
	}
}

// Line is one line of a hunk with its line numbers in the old and new file.
// OldLine is 0 for an added line, and NewLine is 0 for a removed line.
type Line struct {
	Kind    Kind
	Text    string
	OldLine int
	NewLine int
}

// Hunk is one block of changes, introduced by its "@@" header.
type Hunk struct {
	Header string
	Lines  []Line
}

// File holds the changes to one file.
type File struct {
	Path      string
	OldPath   string
	IsNew     bool
	IsDeleted bool
	IsBinary  bool
	Hunks     []Hunk
}

// IsRenamed reports whether the file moved to a new path.
func (f File) IsRenamed() bool {
	return !f.IsNew && !f.IsDeleted && f.OldPath != "" && f.OldPath != f.Path
}

// Stats counts the added and removed lines in the file.
func (f File) Stats() (added, removed int) {
	for _, hunk := range f.Hunks {
		for _, line := range hunk.Lines {
			switch line.Kind {
			case Added:
				added++
			case Removed:
				removed++
			}
		}
	}
	return added, removed
}

// Parse reads a unified diff in git's format and returns its files in order.
// It ignores text that it does not recognize.
func Parse(text string) []File {
	var files []File
	var file *File
	var hunk *Hunk
	oldLine, newLine := 0, 0

	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			files = append(files, File{})
			file = &files[len(files)-1]
			hunk = nil
			file.OldPath, file.Path = gitPaths(line)

		case file == nil:
			continue

		case hunk == nil && strings.HasPrefix(line, "--- "):
			if path := strings.TrimPrefix(line, "--- "); path == "/dev/null" {
				file.IsNew = true
			} else {
				file.OldPath = strings.TrimPrefix(path, "a/")
			}

		case hunk == nil && strings.HasPrefix(line, "+++ "):
			if path := strings.TrimPrefix(line, "+++ "); path == "/dev/null" {
				file.IsDeleted = true
			} else {
				file.Path = strings.TrimPrefix(path, "b/")
			}

		case hunk == nil && strings.HasPrefix(line, "Binary files "):
			file.IsBinary = true

		case strings.HasPrefix(line, "@@"):
			oldLine, newLine = hunkStarts(line)
			file.Hunks = append(file.Hunks, Hunk{Header: line})
			hunk = &file.Hunks[len(file.Hunks)-1]

		// Extended headers, such as "index" and "rename from", come before the first hunk.
		case hunk == nil:
			continue

		case strings.HasPrefix(line, "+"):
			hunk.Lines = append(hunk.Lines, Line{Kind: Added, Text: line[1:], NewLine: newLine})
			newLine++

		case strings.HasPrefix(line, "-"):
			hunk.Lines = append(hunk.Lines, Line{Kind: Removed, Text: line[1:], OldLine: oldLine})
			oldLine++

		// Some tools strip the space from empty context lines.
		case strings.HasPrefix(line, " "), line == "":
			hunk.Lines = append(hunk.Lines, Line{Kind: Context, Text: strings.TrimPrefix(line, " "), OldLine: oldLine, NewLine: newLine})
			oldLine++
			newLine++
		}
	}

	return files
}

// gitPaths reads the old and new path from a "diff --git a/old b/new" line.
// The "---" and "+++" lines replace these paths when the diff has them.
func gitPaths(line string) (oldPath, newPath string) {
	paths := strings.TrimPrefix(line, "diff --git ")
	i := strings.LastIndex(paths, " b/")
	if i < 0 {
		return paths, paths
	}
	return strings.TrimPrefix(paths[:i], "a/"), paths[i+len(" b/"):]
}

// hunkStarts reads the first old and new line numbers from a hunk header,
// such as "@@ -30,6 +31,10 @@".
func hunkStarts(header string) (oldStart, newStart int) {
	fields := strings.Fields(header)
	if len(fields) < 3 {
		return 0, 0
	}
	return rangeStart(fields[1], "-"), rangeStart(fields[2], "+")
}

// rangeStart reads the start of a hunk range, such as 30 from "-30,6".
func rangeStart(field, sign string) int {
	start, _, _ := strings.Cut(strings.TrimPrefix(field, sign), ",")
	n, err := strconv.Atoi(start)
	if err != nil {
		return 0
	}
	return n
}
