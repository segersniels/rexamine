package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testFiles = Files{RexModule: []byte("-- module\n"), Skill: []byte("# skill\n")}

// useTempConfig points the Rex and Claude Code directories at a temporary
// directory and hides any installed rex binary.
func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	t.Setenv("REX_CONFIG", "")
	t.Setenv("PATH", "")
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunInstallsEverything(t *testing.T) {
	dir := useTempConfig(t)
	initPath := filepath.Join(dir, "config", "rex", "init.lua")
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initPath, []byte(`rex.bind("cmd+e", "pane.zoom")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Binary: "/opt/bin/rexdiff", Key: "ctrl+alt+cmd+d", Rex: true, Skill: true, Out: &bytes.Buffer{}}
	for range 2 {
		if err := Run(testFiles, opts); err != nil {
			t.Fatal(err)
		}
	}

	if got := readFile(t, filepath.Join(dir, "config", "rex", "rexdiff.lua")); got != "-- module\n" {
		t.Errorf("Rex module is %q", got)
	}
	if got := readFile(t, filepath.Join(dir, "claude", "skills", "rexdiff", "SKILL.md")); got != "# skill\n" {
		t.Errorf("skill is %q", got)
	}

	init := readFile(t, initPath)
	if !strings.HasPrefix(init, `rex.bind("cmd+e", "pane.zoom")`) {
		t.Errorf("init.lua lost its existing content:\n%s", init)
	}
	if strings.Count(init, rexRequire) != 1 {
		t.Errorf("init.lua loads rexdiff %d times, want once:\n%s", strings.Count(init, rexRequire), init)
	}
	for _, want := range []string{`bin = "/opt/bin/rexdiff"`, `key = "ctrl+alt+cmd+d"`} {
		if !strings.Contains(init, want) {
			t.Errorf("init.lua does not contain %s:\n%s", want, init)
		}
	}
}

func TestRunWithoutAKeyBindsNothing(t *testing.T) {
	dir := useTempConfig(t)

	opts := Options{Binary: "/opt/bin/rexdiff", Rex: true, Out: &bytes.Buffer{}}
	if err := Run(testFiles, opts); err != nil {
		t.Fatal(err)
	}

	if init := readFile(t, filepath.Join(dir, "config", "rex", "init.lua")); strings.Contains(init, "key =") {
		t.Errorf("init.lua binds a key:\n%s", init)
	}
	if _, err := os.Stat(filepath.Join(dir, "claude")); !os.IsNotExist(err) {
		t.Error("the skill was installed without being selected")
	}
}

func TestLuaString(t *testing.T) {
	if got := luaString(`C:\tools\"rexdiff"`); got != `"C:\\tools\\\"rexdiff\""` {
		t.Errorf("got %s", got)
	}
}
