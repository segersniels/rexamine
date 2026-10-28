// Command rexamine shows a diff to review in the terminal: the changes on the
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

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2/compat"

	"github.com/segersniels/rexamine/internal/agent"
	"github.com/segersniels/rexamine/internal/config"
	"github.com/segersniels/rexamine/internal/setup"
	"github.com/segersniels/rexamine/internal/source"
	"github.com/segersniels/rexamine/internal/ui"
	"github.com/segersniels/rexamine/internal/update"
)

// version is set at build time by the release.
var version = "dev"

// The Rex module and the agent skill ship inside the binary, so that
// `rexamine setup` can install them.
var (
	//go:embed rex/rexamine.lua
	rexModule []byte

	//go:embed skills/rexamine/SKILL.md
	skill []byte
)

const usage = `Usage: rexamine [flags] [<pr-number> | <pr-url>]
       rexamine setup [flags]
       rexamine update
       rexamine uninstall

Shows the changes on the current branch compared to the default branch, or
to the branch that --base names.
Give a pull request number or URL to show that pull request instead.

Run "rexamine setup" once to add rexamine to Rex and to install the agent skill.
Run "rexamine update" to install the latest release.
Run "rexamine uninstall" to remove rexamine and everything that setup installed.

Flags:
`

const setupUsage = `Usage: rexamine setup [flags]

Installs the Rex module, loads it from Rex's init.lua with a key that
toggles the rexamine pane, and installs the agent skill for each coding
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
	if len(os.Args) > 1 && os.Args[1] == "update" {
		return runUpdate()
	}
	if len(os.Args) > 1 && os.Args[1] == "uninstall" {
		return runUninstall()
	}

	flags := flag.NewFlagSet("rexamine", flag.ContinueOnError)
	agentPane := flags.String("agent", "", "send comments to the Rex pane with this `block` ID instead of the clipboard")
	uncommitted := flags.Bool("uncommitted", false, "start with uncommitted changes instead of the whole branch")
	base := flags.String("base", "", "compare the branch with this `branch` instead of the default branch, such as the parent of a stacked branch")
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

	src, err := chooseSource(flags.Arg(0), *uncommitted, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexamine:", err)
		return 1
	}

	// An update without `rexamine setup` leaves the old module and skill behind, so each start brings them
	// up to date. A failure only leaves the old files, which `rexamine setup` can fix, so the screen still opens.
	_ = setup.Refresh(setup.Files{RexModule: rexModule, Skill: skill})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var sender agent.Sender = agent.Clipboard{}
	var turns <-chan agent.TurnEvent
	if *agentPane != "" {
		pane := agent.RexPane{Session: os.Getenv("REX_SESSION"), Block: *agentPane}
		sender = pane

		// A pull request also watches, because it can switch to the local changes. Without Rex events, the
		// other views still work.
		turns, _ = pane.WatchTurns(ctx)
	}

	// The compat package asks for the background color at startup, before Bubble Tea takes over the terminal.
	markdownStyle := "light"
	if compat.HasDarkBackground {
		markdownStyle = "dark"
	}

	program := tea.NewProgram(ui.New(src, sender, turns, markdownStyle, config.Load()))

	// The check can wait on the network, so it runs beside the screen and never delays it.
	go func() {
		if notice, ok := update.Check(version); ok {
			program.Send(notice)
		}
	}()
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine:", err)
		return 1
	}
	return 0
}

// chooseSource picks what to show and checks that the tool it needs is
// installed, before the screen opens.
func chooseSource(target string, uncommitted bool, base string) (source.Source, error) {
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
	return source.Local{Scope: scope, Base: base}, nil
}

// runSetup installs the Rex module and the agent skill.
func runSetup(args []string) int {
	flags := flag.NewFlagSet("rexamine setup", flag.ContinueOnError)
	key := flags.String("key", "ctrl+alt+cmd+d", "the `key` that toggles the rexamine pane; empty binds no key")
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
		fmt.Fprintln(os.Stderr, "rexamine setup:", err)
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
		fmt.Fprintln(os.Stderr, "rexamine setup:", err)
		return 1
	}
	return 0
}

// runUpdate installs the latest release in the same way as this installation,
// with npm or with the install script. Both also run `rexamine setup`, so the
// Rex module and the agent skill get the update too.
func runUpdate() int {
	command := update.Command()
	fmt.Println("rexamine: running", command)

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine update:", err)
		return 1
	}
	return 0
}

// runUninstall removes what setup installed, the update check's cache, the
// settings, and then the binary. The binary goes last, so a failed step can run again.
func runUninstall() int {
	if err := setup.Uninstall(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
		return 1
	}

	if err := update.RemoveCache(); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
		return 1
	}

	if err := config.Remove(); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
		return 1
	}

	binary, err := update.Binary()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
		return 1
	}

	// npm also keeps the package and the link on PATH, so npm must remove them.
	if update.InstalledWithNpm(binary) {
		cmd := exec.Command("npm", "uninstall", "-g", "@segersniels/rexamine")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
			return 1
		}
		fmt.Println("✓ Binary: removed the npm package @segersniels/rexamine")
		return 0
	}

	if err := os.Remove(binary); err != nil {
		fmt.Fprintln(os.Stderr, "rexamine uninstall:", err)
		return 1
	}
	fmt.Println("✓ Binary: removed", binary)
	return 0
}

// binaryPath finds the path for Rex to start rexamine with. It returns an
// empty path when the rexamine on PATH is this binary, so that Rex looks it up
// on PATH. Each host then starts its own rexamine, such as on a remote server.
func binaryPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}

	onPath, err := exec.LookPath("rexamine")
	if err != nil {
		return self, nil
	}

	selfInfo, selfErr := os.Stat(self)
	pathInfo, pathErr := os.Stat(onPath)
	if selfErr == nil && pathErr == nil && os.SameFile(selfInfo, pathInfo) {
		return "", nil
	}
	return self, nil
}
