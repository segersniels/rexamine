package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreesListsBranchesWithTheCurrentFirst(t *testing.T) {
	repo := newRepo(t)
	parent := filepath.Dir(repo)
	topic := filepath.Join(parent, "topic-checkout")
	detached := filepath.Join(parent, "detached-checkout")

	for _, args := range [][]string{
		{"worktree", "add", "-q", "-b", "topic", topic},
		{"worktree", "add", "-q", "--detach", detached, "main"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	worktrees, err := Worktrees(topic)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, w := range worktrees {
		names = append(names, w.Name())
	}
	if len(worktrees) != 3 || names[0] != "topic" || names[1] != "feature" || len(names[2]) != 7 {
		t.Fatalf("got worktrees %q, want topic, feature, then a short commit", names)
	}
	if !worktrees[0].Current || worktrees[0].Main {
		t.Errorf("topic should be current and not main: %+v", worktrees[0])
	}
	if !worktrees[1].Main || worktrees[1].Current {
		t.Errorf("feature should be main and not current: %+v", worktrees[1])
	}
}

func TestLocalLoadsTheDirectoryItGets(t *testing.T) {
	repo := newRepo(t)
	t.Chdir(t.TempDir())

	if err := os.WriteFile(filepath.Join(repo, "elsewhere.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	review, err := Local{Scope: Uncommitted, Dir: repo}.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(review.Diff, "b/elsewhere.txt") {
		t.Errorf("diff does not show the file in Dir:\n%s", review.Diff)
	}
}

func TestParseWorktreesSkipsCheckoutsWithoutFiles(t *testing.T) {
	out := "worktree /repo.git\x00bare\x00\x00" +
		"worktree /gone\x00HEAD abc\x00branch refs/heads/gone\x00prunable gitdir file points to non-existent location\x00\x00" +
		"worktree /work\x00HEAD def\x00branch refs/heads/work\x00\x00"

	worktrees := parseWorktrees(out)
	if len(worktrees) != 1 || worktrees[0].Branch != "work" || worktrees[0].Main {
		t.Errorf("got %+v, want only the work branch, not marked main", worktrees)
	}
}
