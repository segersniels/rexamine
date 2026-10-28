package source

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// gitFiles reads the files of a local review from git.
type gitFiles struct {
	root   string
	oldRev string

	// newRev is the version with the changes, or empty for the working tree.
	newRev string
}

// ReadFiles reads oldPaths at oldRev, and newPaths at newRev or in the working tree.
func (g gitFiles) ReadFiles(oldPaths, newPaths []string) (map[string]string, map[string]string) {
	oldTexts := readBlobs(g.root, g.oldRev, oldPaths)
	if g.newRev != "" {
		return oldTexts, readBlobs(g.root, g.newRev, newPaths)
	}

	newTexts := map[string]string{}
	for _, path := range newPaths {
		content, err := os.ReadFile(filepath.Join(g.root, path))
		if err != nil {
			continue
		}
		newTexts[path] = string(content)
	}
	return oldTexts, newTexts
}

// readBlobs reads the files at paths in the git version rev. One git process
// reads them all, because a process per file is slow in a large diff.
func readBlobs(root, rev string, paths []string) map[string]string {
	texts := map[string]string{}
	if len(paths) == 0 {
		return texts
	}

	var input strings.Builder
	for _, path := range paths {
		fmt.Fprintf(&input, "%s:%s\n", rev, path)
	}
	cmd := exec.Command("git", "cat-file", "--batch")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(input.String())
	out, err := cmd.Output()
	if err != nil {
		return texts
	}

	// git answers each path with an "<id> <type> <size>" line and the content
	// with a newline, or with one line that ends in "missing".
	rest := out
	for _, path := range paths {
		header, after, found := bytes.Cut(rest, []byte("\n"))
		if !found {
			break
		}
		rest = after

		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			continue
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		if size+1 > len(rest) {
			break
		}

		// A submodule shows as a commit, which has no text to color.
		if fields[1] == "blob" {
			texts[path] = string(rest[:size])
		}
		rest = rest[size+1:]
	}
	return texts
}
