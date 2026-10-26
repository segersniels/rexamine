<h1 align="center">rexdiff</h1>

<p align="center">
  Review a diff beside your coding agent, and send comments on its lines straight into the agent's prompt.
</p>

<p align="center">
  <a href="https://github.com/segersniels/rexdiff/actions/workflows/ci.yml"><img src="https://github.com/segersniels/rexdiff/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://www.npmjs.com/package/@segersniels/rexdiff"><img src="https://img.shields.io/npm/v/@segersniels/rexdiff" alt="npm"></a>
  <a href="https://github.com/segersniels/rexdiff/blob/main/LICENSE"><img src="https://img.shields.io/github/license/segersniels/rexdiff" alt="MIT license"></a>
</p>

<p align="center">
  <img src="https://github.com/segersniels/rexdiff/raw/main/assets/screenshot.png" alt="rexdiff in a Rex pane beside a Claude Code session" width="100%">
</p>

rexdiff is a terminal diff viewer for [Rex](https://www.superlogical.com/rex). It opens in a pane next to your agent session, such as Claude Code. Select lines, write a question or a remark, and press Enter. rexdiff pastes the file, the line numbers, the selected lines, and your comment into the agent's prompt, so you can keep the conversation in the session you already have.

---

## Quickstart

Install rexdiff, add it to Rex, and install the agent skill with one command:

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/segersniels/rexdiff/main/scripts/install.sh | sh
```

Or install it with npm, then run the setup yourself:

```sh
npm install -g @segersniels/rexdiff
rexdiff setup
```

Focus your agent's pane in Rex and press `ctrl+alt+cmd+d`. rexdiff opens beside it with the changes on your branch. Move to a line, press `c`, write a comment, and press `enter` to send it to the agent.

You can also run rexdiff in any terminal:

```sh
rexdiff                  # the changes on the current branch
rexdiff --uncommitted    # only the uncommitted changes
rexdiff 123              # pull request 123 of the current repository
```

Without an agent pane, rexdiff copies each comment on local changes to the clipboard, so you can paste it into any agent. In a pull request without an agent pane, rexdiff posts each comment on GitHub.

## Features

- **Local changes and pull requests**: review your branch, only the uncommitted changes, or a GitHub pull request with its description, review status, and labels. Press `p` to switch between the pull request and your local changes.
- **Comments into the prompt**: select lines with `v` or the mouse, and send a comment with the lines to the agent pane as a real paste. The agent waits for you to submit, or press `ctrl+s` to submit the comment at once.
- **Comments on GitHub**: in a pull request, post a comment on the selected lines. Beside an agent, press `shift+tab` in the comment box to switch between the agent and GitHub.
- **Live updates**: the view follows your edits, including untracked files.
- **Last agent turn**: inside Rex, press `t` to see only what the agent changed since it started its last turn.
- **Made for narrow panes**: text wraps to the pane's width and reflows when you resize it.
- **Collapsible files**: hide a file with `z` or a click on its title.
- **Light and dark themes** that follow your terminal.

## Docs

- [**Installation and setup**](https://github.com/segersniels/rexdiff/blob/main/docs/install.md)
- [**Usage and keys**](https://github.com/segersniels/rexdiff/blob/main/docs/usage.md)
- [**Rex module and agent skill**](https://github.com/segersniels/rexdiff/blob/main/docs/rex.md)
- [**Contributing**](https://github.com/segersniels/rexdiff/blob/main/CONTRIBUTING.md)

This repository is licensed under the [MIT License](https://github.com/segersniels/rexdiff/blob/main/LICENSE).
