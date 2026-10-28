package editor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestArgsOpenTheLineInEachEditorFamily(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"nvim", []string{"nvim", "+12", "/repo/a.go"}},
		{"/opt/homebrew/bin/vim", []string{"/opt/homebrew/bin/vim", "+12", "/repo/a.go"}},
		{"code --wait", []string{"code", "--wait", "-g", "/repo/a.go:12"}},
		{"cursor", []string{"cursor", "-g", "/repo/a.go:12"}},
		{"zed", []string{"zed", "/repo/a.go:12"}},
		{"hx", []string{"hx", "/repo/a.go:12"}},
		{"ed", []string{"ed", "/repo/a.go"}},
	}
	for _, tt := range tests {
		e := Editor{strings.Fields(tt.command)}
		if got := e.Args("/repo/a.go", 12); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.command, got, tt.want)
		}
	}
}

func TestFindPrefersVisualAndNeedsAnEditorOnThePath(t *testing.T) {
	t.Setenv("VISUAL", "vi")
	t.Setenv("EDITOR", "nano")
	e, err := Find()
	if err != nil || e.name() != "vi" {
		t.Fatalf("got %v, %v; want vi", e, err)
	}

	t.Setenv("VISUAL", "an-editor-that-is-not-installed")
	if _, err := Find(); err == nil {
		t.Error("got no error for an editor that is not installed")
	}

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("SHELL", "")
	if _, err := Find(); err == nil {
		t.Error("got no error without an editor")
	}
}

func TestFindAsksTheInteractiveShell(t *testing.T) {
	// A startup file that prints text, as some do, must not end up in the command.
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("echo hello\nexport EDITOR=vi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	e, err := Find()
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Args("a.go", 1); !slices.Equal(got, []string{"vi", "+1", "a.go"}) {
		t.Errorf("got %q, want vi from .bashrc", got)
	}
}
