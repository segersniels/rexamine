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

func TestLocalOutsideARepository(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := (Local{}).Load(); err == nil {
		t.Error("expected an error outside a git repository")
	}
}
