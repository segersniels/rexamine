# Installation and setup

## Requirements

- macOS or Linux, on arm64 or x64.
- `git` for local changes.
- The [GitHub CLI](https://cli.github.com) (`gh`), signed in, for pull requests.
- [Rex](https://www.superlogical.com/rex) to send comments to an agent pane. Without Rex, comments go to the clipboard.

## Install with the script

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexdiff/main/scripts/install.sh | sh
```

The [script](../scripts/install.sh) downloads the latest release for your OS and CPU, checks it against the release's checksums, installs it to `~/.local/bin`, and runs `rexdiff setup`. It does not change your shell configuration. If `~/.local/bin` is not in your `PATH`, it tells you how to add it.

| Variable              | Effect                                              |
| --------------------- | --------------------------------------------------- |
| `REXDIFF_INSTALL_DIR` | Install the binary to another directory.            |
| `REXDIFF_VERSION`     | Install a specific release, such as `0.2.0`.        |
| `REXDIFF_NO_SETUP=1`  | Skip `rexdiff setup`.                               |

Set a variable for the shell that runs the script:

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexdiff/main/scripts/install.sh | REXDIFF_VERSION=0.2.0 sh
```

To update, run the script again.

## Install the binary yourself

With npm:

```sh
npm install -g @segersniels/rexdiff
```

The npm package installs only the binary for your platform. It needs no install scripts.

With Go 1.24 or later:

```sh
go install github.com/segersniels/rexdiff@latest
```

You can also download an archive for your platform from the [latest release](https://github.com/segersniels/rexdiff/releases/latest). Each archive holds the `rexdiff` binary, the Rex module, and the agent skill.

## Add rexdiff to Rex

```sh
rexdiff setup
```

`rexdiff setup` does three things:

- It writes the Rex module to `~/.config/rex/rexdiff.lua` and loads it from `~/.config/rex/init.lua`, with the full path of the binary and the key `ctrl+alt+cmd+d`.
- It installs the agent skill for each coding agent that it finds. See [Agent skill](rex.md#agent-skill).
- It reloads the Rex configuration.

| Flag         | Effect                                |
| ------------ | ------------------------------------- |
| `--key`      | Bind another key to the pane.         |
| `--key=""`   | Bind no key.                          |
| `--no-rex`   | Skip the Rex module.                  |
| `--no-skill` | Skip the agent skill.                 |

## Update

Install the new version in the same way, then run `rexdiff setup` again to update the Rex module and the agent skill. `rexdiff setup` does not change an `init.lua` that already loads rexdiff.

To change the options of the Rex module, see [Rex module and agent skill](rex.md).
