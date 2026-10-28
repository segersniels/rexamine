package source

import (
	"path/filepath"
	"slices"
	"strings"
)

// Worktree is a checkout of a repository that rexamine can show.
type Worktree struct {
	Path string

	// Branch is the branch that is checked out, or empty for a detached HEAD.
	Branch string
	Head   string

	// Main is set for the main checkout, where the repository was cloned.
	Main bool

	// Current is set for the checkout of the directory that Worktrees got.
	Current bool
}

// Name names the worktree by its branch, or by its commit for a detached HEAD.
func (w Worktree) Name() string {
	if w.Branch != "" {
		return w.Branch
	}
	return w.Head[:min(len(w.Head), 7)]
}

// Worktrees lists the worktrees of the repository in dir, or in the working
// directory when dir is empty. The current worktree comes first, then the main
// checkout, then the others by name. Worktrees without files, such as a bare
// repository or a removed directory, are left out.
func Worktrees(dir string) ([]Worktree, error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root = filepath.Clean(strings.TrimSpace(root))

	out, err := git(dir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}

	worktrees := parseWorktrees(out)
	for i := range worktrees {
		worktrees[i].Current = filepath.Clean(worktrees[i].Path) == root
	}

	slices.SortStableFunc(worktrees, func(a, b Worktree) int {
		if a.Current != b.Current {
			return boolOrder(a.Current)
		}
		if a.Main != b.Main {
			return boolOrder(a.Main)
		}
		return strings.Compare(a.Name(), b.Name())
	})
	return worktrees, nil
}

// boolOrder sorts true before false.
func boolOrder(first bool) int {
	if first {
		return -1
	}
	return 1
}

// parseWorktrees reads the output of git worktree list --porcelain -z. Each
// field ends with a NUL, and an empty field ends a worktree. git lists the
// main checkout first.
func parseWorktrees(out string) []Worktree {
	var worktrees []Worktree
	var current Worktree
	seen, skip := 0, false

	for _, field := range strings.Split(out, "\x00") {
		name, value, _ := strings.Cut(field, " ")
		switch name {
		case "worktree":
			current = Worktree{Path: value, Main: seen == 0}
			seen++
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
		case "bare", "prunable":
			skip = true
		case "":
			if current.Path != "" && !skip {
				worktrees = append(worktrees, current)
			}
			current, skip = Worktree{}, false
		}
	}
	return worktrees
}
