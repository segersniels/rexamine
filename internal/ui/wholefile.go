package ui

import (
	"strings"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"github.com/segersniels/rexamine/internal/diff"
	"github.com/segersniels/rexamine/internal/source"
)

// maxWholeFileSize is the largest file that the page colors in full. A larger
// file, such as a bundle, takes long to parse, so its hunks keep the colors of
// their own lines.
const maxWholeFileSize = 1 << 20

var (
	// wholeFileMu lets one background job at a time use wholeFileHighlighters and versionColorCache.
	wholeFileMu sync.Mutex

	// wholeFileHighlighters color full files in the background, apart from the highlighters of the UI goroutine.
	wholeFileHighlighters = treeSitterHighlighters{}

	// versionColorCache keeps the colors of file versions by path and text, so
	// a reload parses only the files that changed.
	versionColorCache = map[string]versionColors{}

	// latestWholeFilesGen is the generation of the newest background job, so an older job can stop early.
	latestWholeFilesGen atomic.Int64
)

// wholeFileColors holds the syntax colors of the full old and new version of a
// file. A hunk alone can start inside a comment or a JSX element, which
// confuses the parser, while the full file gives it the context.
type wholeFileColors struct {
	oldVersion versionColors
	newVersion versionColors
}

// versionColors holds the lines of one version of a file and their colors.
type versionColors struct {
	lines []string
	spans [][]span
}

// wholeFilesMsg brings the colors of full files, by path, from a background job.
type wholeFilesMsg struct {
	gen    int
	colors map[string]wholeFileColors
}

// apply replaces the colors of each hunk line with the colors of the same line
// in the full file. A line keeps its hunk colors when the file has other text
// at its number, such as a file that changed after the diff loaded.
func (c wholeFileColors) apply(hunk diff.Hunk, hunkLines [][]span) [][]span {
	result := make([][]span, len(hunkLines))
	for i, line := range hunk.Lines {
		result[i] = hunkLines[i]

		version, number := c.newVersion, line.NewLine
		if line.Kind == diff.Removed {
			version, number = c.oldVersion, line.OldLine
		}
		if number < 1 || number > len(version.lines) || version.lines[number-1] != line.Text {
			continue
		}
		result[i] = version.spans[number-1]
	}
	return result
}

// colorWholeFiles reads the full files of the diff and colors them in the
// background, because a large diff takes seconds to parse in full.
func colorWholeFiles(gen int, reader source.FileReader, files []diff.File, markdownStyle string) tea.Cmd {
	latestWholeFilesGen.Store(int64(gen))
	return func() tea.Msg {
		wholeFileMu.Lock()
		defer wholeFileMu.Unlock()

		// While the agent works, reloads queue jobs. The UI drops the result of an older job, so it skips the reads and the parse.
		if int64(gen) != latestWholeFilesGen.Load() {
			return nil
		}

		var oldPaths, newPaths []string
		for _, file := range files {
			// The hunk of a new or deleted file holds the full file, so its colors are correct already.
			if file.IsNew || file.IsDeleted || file.IsBinary || fileColorer(file.Path, markdownStyle, wholeFileHighlighters) == nil {
				continue
			}
			newPaths = append(newPaths, file.Path)

			// Only removed lines show the old version, so a file with only additions does not need it.
			if hasRemovedLines(file) {
				oldPaths = append(oldPaths, oldPath(file))
			}
		}
		oldTexts, newTexts := reader.ReadFiles(oldPaths, newPaths)

		colors := map[string]wholeFileColors{}
		usedColors := map[string]versionColors{}
		for _, file := range files {
			colors[file.Path] = wholeFileColors{
				oldVersion: colorVersion(file.Path, oldTexts[oldPath(file)], markdownStyle, usedColors),
				newVersion: colorVersion(file.Path, newTexts[file.Path], markdownStyle, usedColors),
			}
		}

		// The cache keeps only the versions of this diff, so it does not grow while the agent works.
		versionColorCache = usedColors
		return wholeFilesMsg{gen, colors}
	}
}

// colorVersion colors the text of one version of a file, or reuses the colors
// from the last job. It records the colors in usedColors.
func colorVersion(path, text, markdownStyle string, usedColors map[string]versionColors) versionColors {
	if text == "" || len(text) > maxWholeFileSize {
		return versionColors{}
	}

	key := path + "\x00" + text
	colors, ok := versionColorCache[key]
	if !ok {
		lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		colors = versionColors{lines, highlightLines(fileColorer(path, markdownStyle, wholeFileHighlighters), lines)}
	}
	usedColors[key] = colors
	return colors
}

// hasRemovedLines reports whether a hunk of the file removes a line.
func hasRemovedLines(file diff.File) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind == diff.Removed {
				return true
			}
		}
	}
	return false
}

// oldPath returns the path of the file before the change, which differs after a rename.
func oldPath(file diff.File) string {
	if file.OldPath != "" {
		return file.OldPath
	}
	return file.Path
}
