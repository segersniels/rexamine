// Command rexdiff shows a diff to review in the terminal: the changes on the
// current branch, or a GitHub pull request. Comments on diff lines go to a
// coding agent in another Rex pane, or to the clipboard.
package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/segersniels/rexdiff/internal/agent"
	"github.com/segersniels/rexdiff/internal/setup"
	"github.com/segersniels/rexdiff/internal/source"
	"github.com/segersniels/rexdiff/internal/ui"
)

// version is set at build time by the release.
var version = "dev"

// The Rex module and the agent skill ship inside the binary, so that
// `rexdiff setup` can install them.
var (
	//go:embed rex/rexdiff.lua
	rexModule []byte

	//go:embed skills/rexdiff/SKILL.md
	skill []byte
)

const usage = `Usage: rexdiff [flags] [<pr-number> | <pr-url>]
       rexdiff setup [flags]

Shows the changes on the current branch compared to the default branch.
Give a pull request number or URL to show that pull request instead.

Run "rexdiff setup" once to add rexdiff to Rex and to install the agent skill.

Flags:
`

const setupUsage = `Usage: rexdiff setup [flags]

Installs the Rex module, loads it from Rex's init.lua with a key that
toggles the rexdiff pane, and installs the agent skill for each coding
agent that it finds, such as Claude Code and Codex. Run it again after an
update to update the module and the skill.

Flags:
`

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		return runSetup(os.Args[2:])
	}

	flags := flag.NewFlagSet("rexdiff", flag.ContinueOnError)
	agentPane := flags.String("agent", "", "send comments to the Rex pane with this `block` ID instead of the clipboard")
	uncommitted := flags.Bool("uncommitted", false, "start with uncommitted changes instead of the whole branch")
	showVersion := flags.Bool("version", false, "print the version and exit")
	flags.Usage = func() {
		_, _ = fmt.Fprint(flags.Output(), usage)
		flags.PrintDefaults()
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Println(version)
		return 0
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return 2
	}

	src, err := chooseSource(flags.Arg(0), *uncommitted)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexdiff:", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sender agent.Sender = agent.Clipboard{}
	var turns <-chan struct{}
	if *agentPane != "" {
		pane := agent.RexPane{Session: os.Getenv("REX_SESSION"), Block: *agentPane}
		sender = pane

		// Only local changes can show the agent's last turn. Without Rex events, the other views still work.
		if _, local := src.(source.Local); local {
			turns, _ = pane.WatchTurns(ctx)
		}
	}

	// Ask for the background color before Bubble Tea takes over the terminal.
	markdownStyle := "light"
	if lipgloss.HasDarkBackground() {
		markdownStyle = "dark"
	}

	program := tea.NewProgram(ui.New(src, sender, turns, markdownStyle), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rexdiff:", err)
		return 1
	}
	return 0
}

// chooseSource picks what to show and checks that the tool it needs is
// installed, before the screen opens.
func chooseSource(target string, uncommitted bool) (source.Source, error) {
	if target != "" {
		if _, err := exec.LookPath("gh"); err != nil {
			return nil, errors.New("pull requests need the GitHub CLI: https://cli.github.com")
		}
		return source.PullRequest{Target: target}, nil
	}

	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("local changes need git")
	}

	scope := source.Branch
	if uncommitted {
		scope = source.Uncommitted
	}
	return source.Local{Scope: scope}, nil
}

// runSetup installs the Rex module and the agent skill.
func runSetup(args []string) int {
	flags := flag.NewFlagSet("rexdiff setup", flag.ContinueOnError)
	key := flags.String("key", "ctrl+alt+cmd+d", "the `key` that toggles the rexdiff pane; empty binds no key")
	noRex := flags.Bool("no-rex", false, "do not install the Rex module")
	noSkill := flags.Bool("no-skill", false, "do not install the agent skill")
	flags.Usage = func() {
		_, _ = fmt.Fprint(flags.Output(), setupUsage)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	binary, err := binaryPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexdiff setup:", err)
		return 1
	}

	err = setup.Run(setup.Files{RexModule: rexModule, Skill: skill}, setup.Options{
		Binary: binary,
		Key:    *key,
		Rex:    !*noRex,
		Skill:  !*noSkill,
		Out:    os.Stdout,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexdiff setup:", err)
		return 1
	}
	return 0
}

// binaryPath finds the path for Rex to start rexdiff with. It prefers the
// rexdiff on PATH, such as a package manager's link, because that path stays
// the same across upgrades.
func binaryPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}

	onPath, err := exec.LookPath("rexdiff")
	if err != nil {
		return self, nil
	}

	selfInfo, selfErr := os.Stat(self)
	pathInfo, pathErr := os.Stat(onPath)
	if selfErr == nil && pathErr == nil && os.SameFile(selfInfo, pathInfo) {
		return filepath.Abs(onPath)
	}
	return self, nil
}
