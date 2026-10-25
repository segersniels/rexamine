package source

import (
	"bytes"
	"errors"
	"fmt"
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

// untrackedDiff shows each untracked file as a new file. git diff leaves
// these out.
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

		// git diff --no-index exits with 1 when the files differ, which they always do here.
		patch, err := git(root, "diff", "--no-color", "--no-ext-diff", "--no-index", "--", "/dev/null", path)
		var exitErr *exec.ExitError
		differs := errors.As(err, &exitErr) && exitErr.ExitCode() == 1
		if err != nil && !differs {
			return "", err
		}
		out.WriteString(patch)
	}
	return out.String(), nil
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
