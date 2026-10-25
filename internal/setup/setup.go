// Package setup installs the parts of rexdiff that live outside the binary:
// the Rex module with its key binding, and the agent skill.
package setup

import (
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
	// Binary is the path that Rex starts rexdiff with.
	Binary string

	// Key toggles the rexdiff pane. An empty key binds nothing.
	Key string

	Rex   bool
	Skill bool
	Out   io.Writer
}

// rexRequire is the text that marks an init.lua that already loads rexdiff.
const rexRequire = `require("rexdiff")`

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
	}

	reloadRex(opts.Out)
	return nil
}

// setupCall builds the lines that init.lua needs to load rexdiff.
func setupCall(opts Options) string {
	var call strings.Builder
	call.WriteString("-- Added by `rexdiff setup`.\n")
	call.WriteString(rexRequire + ".setup{\n")
	call.WriteString("  bin = " + luaString(opts.Binary) + ",\n")
	if opts.Key != "" {
		call.WriteString("  key = " + luaString(opts.Key) + ",\n")
	}
	call.WriteString("}\n")
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

// installSkill writes the agent skill to Claude Code's skills directory.
func installSkill(skill []byte, out io.Writer) error {
	dir, err := claudeDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "skills", "rexdiff", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, skill, 0o644); err != nil {
		return err
	}

	report(out, "Agent skill", path)
	return nil
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

// claudeDir finds Claude Code's configuration directory: CLAUDE_CONFIG_DIR,
// or ~/.claude.
func claudeDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
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
