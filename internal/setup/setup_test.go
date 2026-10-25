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

func TestUninstallRemovesEverythingThatRunInstalled(t *testing.T) {
	dir := useTempConfig(t)
	installAgents(t, filepath.Join(dir, "claude"), filepath.Join(dir, ".codex"))
	initPath := filepath.Join(dir, "config", "rex", "init.lua")
	original := `rex.bind("cmd+e", "pane.zoom")` + "\n"
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(initPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := Options{Binary: "/opt/bin/rexdiff", Key: "ctrl+alt+cmd+d", Rex: true, Skill: true, Out: &bytes.Buffer{}}
	if err := Run(testFiles, opts); err != nil {
		t.Fatal(err)
	}

	// An agent that the user removed after setup still had the skill installed.
	if err := os.RemoveAll(filepath.Join(dir, ".codex")); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, initPath); got != original {
		t.Errorf("init.lua is %q, want %q", got, original)
	}
	for _, path := range []string{
		filepath.Join(dir, "config", "rex", "rexdiff.lua"),
		filepath.Join(dir, "claude", "skills", "rexdiff"),
		filepath.Join(dir, ".agents", "skills", "rexdiff"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still exists", path)
		}
	}
}

func TestRemoveSetupCall(t *testing.T) {
	tests := []struct {
		name, text, want string
		removed          bool
	}{
		{
			name:    "current format",
			text:    "a = 1\n\n" + setupCall(Options{Binary: "/bin/rexdiff", Key: "k"}) + "b = 2\n",
			want:    "a = 1\nb = 2\n",
			removed: true,
		},
		{
			name:    "older format",
			text:    "-- Added by `rexdiff setup`.\nrequire(\"rexdiff\").setup{\n  bin = \"/bin/rexdiff\",\n}\n",
			want:    "",
			removed: true,
		},
		{
			name: "lines that the user wrote",
			text: "require(\"rexdiff\").setup{}\n",
			want: "require(\"rexdiff\").setup{}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, removed := removeSetupCall(tt.text)
			if got != tt.want || removed != tt.removed {
				t.Errorf("removeSetupCall() = %q, %v; want %q, %v", got, removed, tt.want, tt.removed)
			}
		})
	}
}
