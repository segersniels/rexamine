// Package setup installs the parts of rexdiff that live outside the binary:
// the Rex module with its key binding, and the skill for each installed agent.
package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Files holds the integration files that ship inside the binary.
type Files struct {
	RexModule []byte
	Skill     []byte
}

// Options selects what to install.
type Options struct {
	// Binary is the path that Rex starts rexdiff with. An empty path lets Rex
	// find rexdiff on PATH.
	Binary string

	// Key toggles the rexdiff pane. An empty key binds nothing.
	Key string

	Rex   bool
	Skill bool
	Out   io.Writer
}

// rexRequire is the text that marks an init.lua that already loads rexdiff.
const rexRequire = `require("rexdiff")`

// rexMarker starts the lines that setup adds to init.lua, so that uninstall
// can find them.
const rexMarker = "-- Added by `rexdiff setup`."

// Run installs the selected parts and reports each step to opts.Out. Running
// it again updates the files and leaves an existing init.lua line alone.
func Run(files Files, opts Options) error {
	if opts.Rex {
		if err := installRex(files.RexModule, opts); err != nil {
			return fmt.Errorf("install the Rex module: %w", err)
		}
	}

	if opts.Skill {
		if err := installSkill(files.Skill, opts.Out); err != nil {
			return fmt.Errorf("install the agent skill: %w", err)
		}
	}
	return nil
}

// Refresh rewrites the installed Rex module and agent skills that differ from
// the files in this binary, so an update that does not run `rexdiff setup`
// still brings them up to date. It only touches files that setup installed,
// and reloads Rex when the module changed.
func Refresh(files Files) error {
	initPath, err := rexInitPath()
	if err != nil {
		return err
	}

	moduleChanged, err := refreshFile(filepath.Join(filepath.Dir(initPath), "rexdiff.lua"), files.RexModule)
	if err != nil {
		return err
	}

	agents, err := knownAgents()
	if err != nil {
		return err
	}

	// Agents that share a skills directory share one copy of the skill.
	seen := map[string]bool{}
	for _, a := range agents {
		if seen[a.skillsDir] {
			continue
		}
		seen[a.skillsDir] = true

		if _, err := refreshFile(filepath.Join(a.skillsDir, "rexdiff", "SKILL.md"), files.Skill); err != nil {
			return err
		}
	}

	if moduleChanged {
		reloadRex(io.Discard)
	}
	return nil
}

// refreshFile replaces the file at path with content when the file exists and
// differs, and reports whether it changed the file.
func refreshFile(path string, content []byte) (bool, error) {
	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if bytes.Equal(current, content) {
		return false, nil
	}
	return true, os.WriteFile(path, content, 0o644)
}

// installRex writes the Rex module next to init.lua, makes init.lua load it,
// and reloads the Rex configuration.
func installRex(module []byte, opts Options) error {
	initPath, err := rexInitPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(initPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	modulePath := filepath.Join(dir, "rexdiff.lua")
	if err := os.WriteFile(modulePath, module, 0o644); err != nil {
		return err
	}
	report(opts.Out, "Rex module", modulePath)

	current, err := os.ReadFile(initPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// Keep the user's own setup call, which may hold other options.
	if strings.Contains(string(current), rexRequire) {
		report(opts.Out, "Rex config", initPath+" already loads rexdiff, left unchanged")
	} else {
		// A blank line keeps the added lines apart from the user's own.
		call := setupCall(opts)
		if len(current) > 0 {
			call = "\n" + call
		}
		if err := appendFile(initPath, call); err != nil {
			return err
		}
		report(opts.Out, "Rex config", initPath)

		// Rex finds rexdiff on PATH, but a pane does not get what ~/.zshrc adds to PATH.
		if opts.Binary == "" {
			_, _ = fmt.Fprintln(opts.Out, "  Rex starts rexdiff from a non-interactive shell, which does not read ~/.zshrc.\n  If the pane says \"command not found\", add the directory of rexdiff to PATH in ~/.zshenv.\n  See https://github.com/segersniels/rexdiff/blob/main/docs/install.md#put-rexdiff-on-the-path-that-rex-uses")
		}
	}

	reloadRex(opts.Out)
	return nil
}

// setupCall builds the lines that init.lua needs to load rexdiff. A Lua error
// stops Rex from running the rest of init.lua, so the lines catch an error in
// the module, for example after a Rex update, and log it instead.
func setupCall(opts Options) string {
	var call strings.Builder
	call.WriteString(rexMarker + " An error in rexdiff does not stop the rest of this file.\n")
	call.WriteString("do\n")
	call.WriteString("  local ok, err = pcall(function()\n")
	call.WriteString("    " + rexRequire + ".setup{\n")
	if opts.Binary != "" {
		call.WriteString("      bin = " + luaString(opts.Binary) + ",\n")
	}
	if opts.Key != "" {
		call.WriteString("      key = " + luaString(opts.Key) + ",\n")
	}
	call.WriteString("    }\n")
	call.WriteString("  end)\n")
	call.WriteString(`  if not ok then rex.log("error", "rexdiff: " .. tostring(err)) end` + "\n")
	call.WriteString("end\n")
	return call.String()
}

// reloadRex asks a running Rex server to load the new configuration. Without
// Rex, it tells the user to reload later.
func reloadRex(out io.Writer) {
	if _, err := exec.LookPath("rex"); err != nil {
		_, _ = fmt.Fprintln(out, "  Run `rex config reload` after you install Rex.")
		return
	}

	if output, err := exec.Command("rex", "config", "reload").CombinedOutput(); err != nil {
		_, _ = fmt.Fprintf(out, "  Could not reload Rex: %s\n  Run `rex config reload` when Rex is running.\n", strings.TrimSpace(string(output)))
		return
	}
	report(out, "Rex", "configuration reloaded")
}

// installSkill writes the agent skill to the skills directory of every
// installed agent. Agents that share a directory get the skill once.
func installSkill(skill []byte, out io.Writer) error {
	agents, err := installedAgents()
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		report(out, "Agent skill", "no supported agent found, skipped")
		return nil
	}

	var dirs []string
	names := map[string][]string{}
	for _, a := range agents {
		if names[a.skillsDir] == nil {
			dirs = append(dirs, a.skillsDir)
		}
		names[a.skillsDir] = append(names[a.skillsDir], a.name)
	}

	for _, dir := range dirs {
		path := filepath.Join(dir, "rexdiff", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, skill, 0o644); err != nil {
			return err
		}
		report(out, "Agent skill", fmt.Sprintf("%s (%s)", path, strings.Join(names[dir], ", ")))
	}
	return nil
}

// Uninstall removes the Rex module, the lines that setup added to init.lua,
// and the agent skill of every known agent, and reports each step to out.
func Uninstall(out io.Writer) error {
	if err := uninstallRex(out); err != nil {
		return fmt.Errorf("remove the Rex module: %w", err)
	}

	if err := uninstallSkill(out); err != nil {
		return fmt.Errorf("remove the agent skill: %w", err)
	}
	return nil
}

// uninstallRex removes the Rex module and the setup lines in init.lua, and
// reloads the Rex configuration when it changed something.
func uninstallRex(out io.Writer) error {
	initPath, err := rexInitPath()
	if err != nil {
		return err
	}

	changed := false
	modulePath := filepath.Join(filepath.Dir(initPath), "rexdiff.lua")
	err = os.Remove(modulePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err == nil {
		report(out, "Rex module", "removed "+modulePath)
		changed = true
	}

	current, err := os.ReadFile(initPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	updated, removed := removeSetupCall(string(current))
	if removed {
		if err := os.WriteFile(initPath, []byte(updated), 0o644); err != nil {
			return err
		}
		report(out, "Rex config", "removed rexdiff from "+initPath)
		changed = true
	}

	// Lines that the user wrote can hold other options, so only the user can remove them.
	if strings.Contains(updated, rexRequire) {
		_, _ = fmt.Fprintf(out, "  %s still loads rexdiff. Remove those lines yourself.\n", initPath)
	}

	if changed {
		reloadRex(out)
	}
	return nil
}

// removeSetupCall removes the lines that setup added to init.lua, and the
// blank line that setup put above them. The lines end at the first line that
// is "end", or "}" in the older format without pcall.
func removeSetupCall(text string) (string, bool) {
	start := strings.Index(text, rexMarker)
	if start < 0 {
		return text, false
	}

	lines := strings.SplitAfter(text[start:], "\n")
	end := start
	for _, line := range lines {
		end += len(line)
		if trimmed := strings.TrimRight(line, "\r\n"); trimmed == "end" || trimmed == "}" {
			return strings.TrimSuffix(text[:start], "\n") + text[end:], true
		}
	}
	return text, false
}

// uninstallSkill removes the agent skill from the skills directory of every
// known agent, also of agents that are no longer installed.
func uninstallSkill(out io.Writer) error {
	agents, err := knownAgents()
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, a := range agents {
		if seen[a.skillsDir] {
			continue
		}
		seen[a.skillsDir] = true

		dir := filepath.Join(a.skillsDir, "rexdiff")
		err := os.Remove(filepath.Join(dir, "SKILL.md"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}

		// The directory stays when it holds files that the user added.
		_ = os.Remove(dir)
		report(out, "Agent skill", "removed "+dir)
	}
	return nil
}

// agent is a coding agent that loads skills from a directory.
type agent struct {
	name string

	// configDir exists when the agent is installed.
	configDir string
	skillsDir string
}

// installedAgents lists the agents whose configuration directory exists.
func installedAgents() ([]agent, error) {
	known, err := knownAgents()
	if err != nil {
		return nil, err
	}

	var installed []agent
	for _, a := range known {
		if info, err := os.Stat(a.configDir); err == nil && info.IsDir() {
			installed = append(installed, a)
		}
	}
	return installed, nil
}

// knownAgents lists the agents that rexdiff can install the skill for. The
// directories follow the skills CLI, github.com/vercel-labs/skills.
func knownAgents() ([]agent, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	config := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	claude := envOr("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	codex := envOr("CODEX_HOME", filepath.Join(home, ".codex"))

	// Several agents read the shared Agent Skills directory.
	shared := filepath.Join(home, ".agents", "skills")

	return []agent{
		{"Claude Code", claude, filepath.Join(claude, "skills")},
		{"Codex", codex, shared},
		{"OpenCode", filepath.Join(config, "opencode"), filepath.Join(config, "opencode", "skills")},
		{"Gemini CLI", filepath.Join(home, ".gemini"), filepath.Join(home, ".gemini", "skills")},
		{"GitHub Copilot", filepath.Join(home, ".copilot"), filepath.Join(home, ".copilot", "skills")},
		{"Cursor", filepath.Join(home, ".cursor"), filepath.Join(home, ".cursor", "skills")},
		{"Amp", filepath.Join(config, "amp"), filepath.Join(config, "agents", "skills")},
		{"Pi", filepath.Join(home, ".pi", "agent"), shared},
		{"Droid", filepath.Join(home, ".factory"), filepath.Join(home, ".factory", "skills")},
		{"Goose", filepath.Join(config, "goose"), filepath.Join(config, "goose", "skills")},
		{"Crush", filepath.Join(config, "crush"), filepath.Join(config, "crush", "skills")},
	}, nil
}

// envOr returns the environment variable, or fallback when it is empty.
func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

// rexInitPath finds Rex's configuration file, as Rex does: REX_CONFIG, then
// $XDG_CONFIG_HOME/rex/init.lua, then ~/.config/rex/init.lua.
func rexInitPath() (string, error) {
	if path := os.Getenv("REX_CONFIG"); path != "" {
		return path, nil
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "rex", "init.lua"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rex", "init.lua"), nil
}

// appendFile adds text to the end of a file and creates the file if needed.
func appendFile(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(text); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// luaString quotes text as a Lua string literal.
func luaString(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}

// report prints one finished step.
func report(out io.Writer, what, detail string) {
	_, _ = fmt.Fprintf(out, "✓ %s: %s\n", what, detail)
}
