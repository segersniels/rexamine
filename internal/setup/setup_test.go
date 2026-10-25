package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testFiles = Files{RexModule: []byte("-- module\n"), Skill: []byte("# skill\n")}

// useTempConfig points the home directory and the Rex and agent directories
// at a temporary directory, and hides any installed rex binary.
func useTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(dir, "claude"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("REX_CONFIG", "")
	t.Setenv("PATH", "")
	return dir
}

// installAgents creates the configuration directories that mark agents as installed.
func installAgents(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
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
	installAgents(t, filepath.Join(dir, "claude"))
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
	for _, want := range []string{`bin = "/opt/bin/rexdiff"`, `key = "ctrl+alt+cmd+d"`, "pcall(function()"} {
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

func TestSkillGoesToEveryInstalledAgent(t *testing.T) {
	dir := useTempConfig(t)

	// Codex and Pi share ~/.agents/skills. Gemini CLI is not installed.
	installAgents(t, filepath.Join(dir, "claude"), filepath.Join(dir, ".codex"), filepath.Join(dir, ".pi", "agent"), filepath.Join(dir, "config", "opencode"))

	out := &bytes.Buffer{}
	if err := Run(testFiles, Options{Skill: true, Out: out}); err != nil {
		t.Fatal(err)
	}

	for _, skillsDir := range []string{
		filepath.Join(dir, "claude", "skills"),
		filepath.Join(dir, ".agents", "skills"),
		filepath.Join(dir, "config", "opencode", "skills"),
	} {
		if got := readFile(t, filepath.Join(skillsDir, "rexdiff", "SKILL.md")); got != "# skill\n" {
			t.Errorf("skill in %s is %q", skillsDir, got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".gemini")); !os.IsNotExist(err) {
		t.Error("the skill was installed for Gemini CLI, which is not installed")
	}
	if !strings.Contains(out.String(), "(Codex, Pi)") {
		t.Errorf("output does not name both agents of the shared directory:\n%s", out)
	}
}

func TestSkillWithoutAnAgentInstallsNothing(t *testing.T) {
	dir := useTempConfig(t)

	out := &bytes.Buffer{}
	if err := Run(testFiles, Options{Skill: true, Out: out}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents")); !os.IsNotExist(err) {
		t.Error("the skill was installed without an agent")
	}
	if !strings.Contains(out.String(), "no supported agent found") {
		t.Errorf("output does not say that no agent was found:\n%s", out)
	}
}

func TestLuaString(t *testing.T) {
	if got := luaString(`C:\tools\"rexdiff"`); got != `"C:\\tools\\\"rexdiff\""` {
		t.Errorf("got %s", got)
	}
}
