# Installation and setup

## Requirements

- macOS or Linux, on arm64 or x64.
- `git` for local changes.
- The [GitHub CLI](https://cli.github.com) (`gh`), signed in, for pull requests.
- [Rex](https://www.superlogical.com/rex) to send comments to an agent pane. Without Rex, comments go to the clipboard.

## Install the binary

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
- It installs the agent skill to `~/.claude/skills/rexdiff`.
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
