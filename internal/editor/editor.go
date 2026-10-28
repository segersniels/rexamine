// Package editor opens a file at a line in the user's editor of choice.
package editor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Editor is the editor that $VISUAL or $EDITOR names, with its own flags.
type Editor struct {
	command []string
}

// windowedEditors open their own window, so rexamine starts them and keeps
// running. Other editors need the terminal.
var windowedEditors = map[string]bool{
	"code": true, "code-insiders": true, "codium": true, "cursor": true,
	"windsurf": true, "zed": true, "subl": true,
}

// shellTimeout limits how long the interactive shell may take to start.
const shellTimeout = 3 * time.Second

// Find finds the editor in $VISUAL, then in $EDITOR. When both are empty, it
// asks the user's interactive shell, because Rex starts rexamine without the
// interactive setup, such as ~/.zshrc, where many users set $EDITOR. An
// editor that is not on the PATH counts as not found.
func Find() (Editor, error) {
	value := os.Getenv("VISUAL")
	if value == "" {
		value = os.Getenv("EDITOR")
	}
	if value == "" {
		value = shellEditor()
	}

	command := strings.Fields(value)
	if len(command) == 0 {
		return Editor{}, errors.New("set $EDITOR to open files, such as EDITOR=nvim or EDITOR=code")
	}
	if _, err := exec.LookPath(command[0]); err != nil {
		return Editor{}, err
	}
	return Editor{command}, nil
}

// shellEditor asks the interactive shell in $SHELL for $VISUAL or $EDITOR, or
// returns an empty string when it cannot tell.
func shellEditor() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()

	// The shell's startup files can print text of their own, so a marker shows where the answer starts.
	const marker = "__rexamine_editor__"
	cmd := exec.CommandContext(ctx, shell, "-ic", `printf '`+marker+`%s' "${VISUAL:-$EDITOR}"`)

	// An interactive shell takes over the terminal it finds, which leaves rexamine unable to read keys. A new
	// session has no terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	_, value, _ := strings.Cut(string(out), marker)
	return strings.TrimSpace(value)
}

// name is the editor's program name without its directory, such as "nvim".
func (e Editor) name() string {
	return filepath.Base(e.command[0])
}

// InTerminal reports whether the editor runs in the terminal, such as nvim,
// instead of in its own window.
func (e Editor) InTerminal() bool {
	return !windowedEditors[e.name()]
}

// Args builds the command that opens path at line. Each editor family takes
// the line in its own way. An unknown editor opens the file at its start.
func (e Editor) Args(path string, line int) []string {
	args := append([]string{}, e.command...)
	number := strconv.Itoa(line)

	switch e.name() {
	case "vi", "vim", "nvim", "nano", "emacs", "emacsclient", "micro", "kak":
		return append(args, "+"+number, path)
	case "code", "code-insiders", "codium", "cursor", "windsurf":
		return append(args, "-g", path+":"+number)
	case "zed", "subl", "hx", "helix":
		return append(args, path+":"+number)
	}
	return append(args, path)
}
