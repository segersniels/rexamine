package source

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Scope selects which local changes to show.
type Scope int

// The local scopes.
const (
	// Branch shows every change since the branch split from the default
	// branch, including uncommitted and untracked files.
	Branch Scope = iota

	// Uncommitted shows only changes since the last commit, including
	// untracked files.
	Uncommitted
)

// Next returns the other scope.
func (s Scope) Next() Scope {
	if s == Branch {
		return Uncommitted
	}
	return Branch
}

// String names the scope for messages.
func (s Scope) String() string {
	if s == Uncommitted {
		return "uncommitted changes"
	}
	return "branch changes"
}

// Local loads the changes in the git repository of the working directory.
type Local struct {
	Scope Scope
}

// Load runs git and returns the changes in the scope.
func (l Local) Load() (Review, error) {
	root, err := git("", "rev-parse", "--show-toplevel")
	if err != nil {
		return Review{}, errors.New("not inside a git repository")
	}
	root = strings.TrimSpace(root)

	branch, err := git(root, "branch", "--show-current")
	if err != nil {
		return Review{}, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "detached HEAD"
	}

	base, baseName, headName := "HEAD", "last commit", "working tree"

	// A repository without commits has nothing to compare against, so every file shows as new.
	if _, err := git(root, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		emptyTree, err := git(root, "hash-object", "-t", "tree", "/dev/null")
		if err != nil {
			return Review{}, err
		}
		base, baseName = strings.TrimSpace(emptyTree), "no commits"
	} else if l.Scope == Branch {
		defaultBranch, err := defaultBranch(root)
		if err != nil {
			return Review{}, err
		}

		mergeBase, err := git(root, "merge-base", "HEAD", defaultBranch)
		if err != nil {
			return Review{}, fmt.Errorf("find where %s split from %s: %w", branch, defaultBranch, err)
		}
		base, baseName, headName = strings.TrimSpace(mergeBase), defaultBranch, branch
	}

	tracked, err := git(root, "diff", "--no-color", "--no-ext-diff", "-M", base)
	if err != nil {
		return Review{}, err
	}

	untracked, err := untrackedDiff(root)
	if err != nil {
		return Review{}, err
	}

	title := "Changes on " + branch
	if l.Scope == Uncommitted {
		title = "Uncommitted changes on " + branch
	}
	return Review{
		Context: filepath.Base(root),
		Title:   title,
		Base:    baseName,
		Head:    headName,
		Diff:    tracked + untracked,
	}, nil
}

// defaultBranch finds the branch that pull requests merge into, preferring
// the remote's default branch over a local one.
func defaultBranch(root string) (string, error) {
	if ref, err := git(root, "rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
		return strings.TrimSpace(ref), nil
	}

	for _, candidate := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := git(root, "rev-parse", "--verify", "--quiet", candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("cannot find the default branch; set it with: git remote set-head origin --auto")
}

// binaryCheckSize is how much of a file git reads to decide that it is binary.
const binaryCheckSize = 8000

// untrackedDiff shows each untracked file as a new file. git diff leaves
// these out. One git process per file takes seconds in a repository with many
// untracked files, so the diff is written here instead.
func untrackedDiff(root string) (string, error) {
	list, err := git(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}

	var out strings.Builder
	for _, path := range strings.Split(strings.TrimRight(list, "\x00"), "\x00") {
		if path == "" {
			continue
		}

		err := writeNewFileDiff(&out, root, path)

		// Editors create and remove temporary files all the time, so a file can be gone already.
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

// writeNewFileDiff writes the diff that git diff --no-index prints for a new
// file, without the index line.
func writeNewFileDiff(out *strings.Builder, root, path string) error {
	full := filepath.Join(root, path)
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}

	// A symbolic link stores the path it points to.
	mode := "100644"
	var content []byte
	if info.Mode()&fs.ModeSymlink != 0 {
		mode = "120000"
		target, err := os.Readlink(full)
		if err != nil {
			return err
		}
		content = []byte(target)
	} else {
		if info.Mode()&0o111 != 0 {
			mode = "100755"
		}
		content, err = os.ReadFile(full)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "diff --git a/%s b/%s\nnew file mode %s\n", path, path, mode)
	if len(content) == 0 {
		return nil
	}
	if bytes.IndexByte(content[:min(len(content), binaryCheckSize)], 0) >= 0 {
		fmt.Fprintf(out, "Binary files /dev/null and b/%s differ\n", path)
		return nil
	}

	text := string(content)
	endsWithNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	lineRange := "1"
	if len(lines) != 1 {
		lineRange = fmt.Sprintf("1,%d", len(lines))
	}
	fmt.Fprintf(out, "--- /dev/null\n+++ b/%s\n@@ -0,0 +%s @@\n", path, lineRange)
	for _, line := range lines {
		out.WriteString("+" + line + "\n")
	}
	if !endsWithNewline {
		out.WriteString("\\ No newline at end of file\n")
	}
	return nil
}

// gitError keeps git's own message next to its exit status.
type gitError struct {
	args    []string
	message string
	err     error
}

func (e *gitError) Error() string {
	return fmt.Sprintf("git %s: %s", e.args[0], e.message)
}

func (e *gitError) Unwrap() error { return e.err }

// git runs a git command in dir and returns what it printed.
func git(dir string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return string(out), &gitError{args: args, message: message, err: err}
	}
	return string(out), nil
}
