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

	// LastTurn shows the changes since the agent started its last turn.
	LastTurn
)

// Next returns the scope after s, in the order branch, uncommitted, last turn.
func (s Scope) Next() Scope {
	return (s + 1) % 3
}

// String names the scope for messages.
func (s Scope) String() string {
	switch s {
	case Uncommitted:
		return "uncommitted changes"
	case LastTurn:
		return "last agent turn"
	default:
		return "branch changes"
	}
}

// Local loads the changes in the git repository of the working directory.
type Local struct {
	Scope Scope

	// TurnStart is the snapshot of the working tree from when the agent
	// started its latest turn, or empty before the first turn.
	TurnStart string

	// PreviousTurnStart is the snapshot from the start of the last turn before
	// the latest one that changed files, or empty if there was none.
	PreviousTurnStart string
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

	if l.Scope == LastTurn {
		return l.loadLastTurn(root, branch)
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

// loadLastTurn compares the snapshot from the start of the agent's last turn
// with the working tree now.
func (l Local) loadLastTurn(root, branch string) (Review, error) {
	review := Review{
		Context: filepath.Base(root),
		Title:   "Last agent turn on " + branch,
		Base:    "turn start",
		Head:    "working tree",
	}

	waiting := review
	waiting.Title = "No agent changes yet on " + branch

	// Before the first turn there is nothing to compare, so the page shows no changes.
	if l.TurnStart == "" {
		return waiting, nil
	}

	now, err := snapshot(root)
	if err != nil {
		return Review{}, err
	}

	// A turn without edits, such as an answer to a question, keeps the changes of the turn before it in view.
	base := l.TurnStart
	if now == l.TurnStart {
		base = l.PreviousTurnStart
	}
	if base == "" {
		return waiting, nil
	}

	review.Diff, err = git(root, "diff", "--no-color", "--no-ext-diff", "-M", base, now)
	if err != nil {
		return Review{}, err
	}
	return review, nil
}

// Snapshot saves the working tree of the repository in the working directory,
// with untracked files, and returns the ID of the saved tree.
func Snapshot() (string, error) {
	root, err := git("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.New("not inside a git repository")
	}
	return snapshot(strings.TrimSpace(root))
}

// snapshot adds every file to a copy of the index and writes it as a tree.
// The real index stays as it is, so the user's staged changes do not change.
func snapshot(root string) (string, error) {
	dir, err := os.MkdirTemp("", "rexdiff-index-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	index := filepath.Join(dir, "index")
	if err := copyIndex(root, index); err != nil {
		return "", err
	}
	env := []string{"GIT_INDEX_FILE=" + index}

	// With --ignore-errors, git exits with 1 when it cannot add a file, such as
	// a temporary file that an editor removed. It still adds the other files.
	_, err = gitEnv(root, env, "add", "-A", "--ignore-errors")
	var exitErr *exec.ExitError
	addedSome := errors.As(err, &exitErr) && exitErr.ExitCode() == 1
	if err != nil && !addedSome {
		return "", err
	}

	tree, err := gitEnv(root, env, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(tree), nil
}

// copyIndex copies the index of the repository to dst, so git only reads the
// files that changed since the index was written. A repository without an
// index has nothing to copy, and git reads a missing index as empty.
func copyIndex(root, dst string) error {
	src, err := git(root, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return err
	}
	src = strings.TrimSpace(src)

	content, err := os.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, content, 0o600); err != nil {
		return err
	}

	// git compares the time of the index with the times of the files to find
	// files that changed right after the index was written, so keep it.
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
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
	return gitEnv(dir, nil, args...)
}

// gitEnv runs a git command in dir with extra environment variables, such as
// GIT_INDEX_FILE, and returns what it printed.
func gitEnv(dir string, env []string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
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
