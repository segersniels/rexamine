# Installation and setup

## Requirements

- macOS or Linux, on arm64 or x64.
- `git` for local changes.
- The [GitHub CLI](https://cli.github.com) (`gh`), signed in, for pull requests.
- [Rex](https://www.superlogical.com/rex) to send comments to an agent pane. Without an agent pane, comments on local changes go to the clipboard, and comments on a pull request go to GitHub.

## Install with the script

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexamine/main/scripts/install.sh | sh
```

The [script](../scripts/install.sh) downloads the latest release for your OS and CPU, checks it against the release's checksums, installs it to `~/.local/bin`, and runs `rexamine setup`. It does not change your shell configuration. If `~/.local/bin` is not in your `PATH`, it tells you how to add it.

| Variable              | Effect                                              |
| --------------------- | --------------------------------------------------- |
| `REXAMINE_INSTALL_DIR` | Install the binary to another directory.            |
| `REXAMINE_VERSION`     | Install a specific release, such as `0.2.0`.        |
| `REXAMINE_NO_SETUP=1`  | Skip `rexamine setup`.                               |

Set a variable for the shell that runs the script:

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexamine/main/scripts/install.sh | REXAMINE_VERSION=0.2.0 sh
```

To update, run `rexamine update` or run the script again.

## Install the binary yourself

With npm:

```sh
npm install -g @segersniels/rexamine
```

The npm package installs only the binary for your platform. It needs no install scripts.

With Go 1.24 or later:

```sh
go install github.com/segersniels/rexamine@latest
```

You can also download an archive for your platform from the [latest release](https://github.com/segersniels/rexamine/releases/latest). Each archive holds the `rexamine` binary, the Rex module, and the agent skill.

## Put rexamine on the PATH that Rex uses

Rex starts rexamine from a non-interactive shell. That shell does not read `~/.zshrc`, so rexamine can work in a normal pane, but the rexamine pane says `command not found`. Add the directory of rexamine to `PATH` in a file that every shell reads. For zsh, use `~/.zshenv`:

```sh
echo 'export PATH="/path/to/dir:$PATH"' >> ~/.zshenv
```

Find the directory with `dirname "$(command -v rexamine)"`. It depends on how you installed rexamine:

| Install method | Directory                                                                             |
| -------------- | ------------------------------------------------------------------------------------- |
| Script         | `~/.local/bin`, or `REXAMINE_INSTALL_DIR`                                              |
| npm            | The `bin` directory of npm's prefix: `"$(npm prefix -g)/bin"`. With a Node version manager, such as nvm or mise, it changes with the Node version. |
| Go             | `"$(go env GOPATH)/bin"`, usually `~/go/bin`                                          |
| Archive        | The directory where you put the binary                                                |

Do this on each host where you run Rex sessions, such as a remote server. Rex starts the pane on the host of the session, so each host needs its own rexamine.

To check the setup, run rexamine from a clean, non-interactive shell:

```sh
env -i HOME="$HOME" "$SHELL" -lc 'command -v rexamine'
```

When this prints nothing, Rex cannot find rexamine either.

## Add rexamine to Rex

```sh
rexamine setup
```

`rexamine setup` does three things:

- It writes the Rex module to `~/.config/rex/rexamine.lua` and loads it from `~/.config/rex/init.lua` with the key `ctrl+alt+cmd+d`. When rexamine is on your `PATH`, Rex starts it from there. If not, setup adds the full path of the binary.
- It installs the agent skill for each coding agent that it finds. See [Agent skill](rex.md#agent-skill).
- It reloads the Rex configuration.

| Flag         | Effect                                |
| ------------ | ------------------------------------- |
| `--key`      | Bind another key to the pane.         |
| `--key=""`   | Bind no key.                          |
| `--no-rex`   | Skip the Rex module.                  |
| `--no-skill` | Skip the agent skill.                 |

## Update

Run `rexamine update`. It finds out how you installed rexamine and installs the latest release in the same way: with npm for an npm install, or with the install script for other installations. Then it runs `rexamine setup` to update the Rex module and the agent skill. `rexamine setup` does not change an `init.lua` that already loads rexamine.

rexamine checks GitHub for a new release once an hour. When one is available, the footer tells you to run `rexamine update`.

To change the options of the Rex module, see [Rex module and agent skill](rex.md).

## Uninstall

```sh
rexamine uninstall
```

`rexamine uninstall` removes everything that rexamine installed:

- The Rex module and the lines that `rexamine setup` added to `init.lua`. Then it reloads the Rex configuration. If you wrote your own lines to load rexamine, it tells you to remove them yourself.
- The agent skill, for every supported agent, also for agents that you removed.
- The cache of the update check.
- The saved settings in `~/.config/rexamine`.
- The binary. For an npm install, it runs `npm uninstall -g @segersniels/rexamine`.
