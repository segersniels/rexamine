package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a repository with one commit on main and a feature branch
// that has one commit, one uncommitted change, and one untracked file.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	write("base.txt", "base\n")
	run("add", ".")
	run("commit", "-q", "-m", "base")

	run("checkout", "-q", "-b", "feature")
	write("committed.txt", "committed\n")
	run("add", ".")
	run("commit", "-q", "-m", "feature")

	write("base.txt", "base\nedited\n")
	write("untracked.txt", "new\n")
	return dir
}

func TestLocalScopes(t *testing.T) {
	t.Chdir(newRepo(t))

	tests := []struct {
		scope   Scope
		want    []string
		notWant []string
	}{
		{Branch, []string{"committed.txt", "base.txt", "untracked.txt"}, nil},
		{Uncommitted, []string{"base.txt", "untracked.txt"}, []string{"committed.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.scope.String(), func(t *testing.T) {
			review, err := Local{Scope: tt.scope}.Load()
			if err != nil {
				t.Fatal(err)
			}

			for _, path := range tt.want {
				if !strings.Contains(review.Diff, "b/"+path) {
					t.Errorf("diff does not show %s:\n%s", path, review.Diff)
				}
			}
			for _, path := range tt.notWant {
				if strings.Contains(review.Diff, "b/"+path) {
					t.Errorf("diff shows %s:\n%s", path, review.Diff)
				}
			}
			if !strings.Contains(review.Title, "feature") {
				t.Errorf("title %q does not name the branch", review.Title)
			}
		})
	}
}

func TestLocalWithoutCommits(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, name := range []string{"staged.txt", "untracked.txt"} {
		if err := os.WriteFile(name, []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "staged.txt"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	for _, scope := range []Scope{Branch, Uncommitted} {
		t.Run(scope.String(), func(t *testing.T) {
			review, err := Local{Scope: scope}.Load()
			if err != nil {
				t.Fatal(err)
			}

			for _, path := range []string{"staged.txt", "untracked.txt"} {
				if !strings.Contains(review.Diff, "b/"+path) {
					t.Errorf("diff does not show %s:\n%s", path, review.Diff)
				}
			}
		})
	}
}

func TestUntrackedDiffMatchesGit(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"text.txt":       "a\nb\n",
		"no-newline.txt": "x",
		"empty.txt":      "",
		"blank-line.txt": "\n",
		"binary.dat":     "a\x00b",
		"with space.txt": "one\n",
		"dir/nested.txt": "nested\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("text.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	got, err := untrackedDiff(dir)
	if err != nil {
		t.Fatal(err)
	}

	paths, err := git(dir, "ls-files", "--others", "-z")
	if err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	for _, path := range strings.Split(strings.TrimRight(paths, "\x00"), "\x00") {
		patch, _ := git(dir, "diff", "--no-color", "--no-ext-diff", "--no-index", "--", "/dev/null", path)
		want.WriteString(patch)
	}

	// The parser ignores index lines, and git ends a "+++" path that has a space with a tab.
	var lines []string
	for _, line := range strings.Split(want.String(), "\n") {
		if !strings.HasPrefix(line, "index ") {
			lines = append(lines, strings.TrimSuffix(line, "\t"))
		}
	}
	if want := strings.Join(lines, "\n"); got != want {
		t.Errorf("diff differs from git:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestLocalOutsideARepository(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := (Local{}).Load(); err == nil {
		t.Error("expected an error outside a git repository")
	}
}

func TestLastTurnShowsOnlyTheChangesSinceTheTurnStarted(t *testing.T) {
	t.Chdir(newRepo(t))

	start, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	// The agent edits a changed file and creates a new one during its turn.
	if err := os.WriteFile("base.txt", []byte("base\nedited\nby agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("agent.txt", []byte("agent\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	review, err := Local{Scope: LastTurn, TurnStart: start}.Load()
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"b/agent.txt", "+by agent"} {
		if !strings.Contains(review.Diff, want) {
			t.Errorf("diff does not show %q:\n%s", want, review.Diff)
		}
	}
	for _, notWant := range []string{"committed.txt", "untracked.txt", "+edited"} {
		if strings.Contains(review.Diff, notWant) {
			t.Errorf("diff shows %q from before the turn:\n%s", notWant, review.Diff)
		}
	}

	// The snapshot uses its own index, so the user's staged changes stay as they are.
	if staged, err := git("", "diff", "--cached", "--name-only"); err != nil || staged != "" {
		t.Errorf("snapshot changed the index: %q, %v", staged, err)
	}
}

func TestLastTurnBeforeTheFirstTurnShowsNoChanges(t *testing.T) {
	t.Chdir(newRepo(t))

	review, err := Local{Scope: LastTurn}.Load()
	if err != nil {
		t.Fatal(err)
	}
	if review.Diff != "" {
		t.Errorf("expected no changes before the first turn:\n%s", review.Diff)
	}
}
