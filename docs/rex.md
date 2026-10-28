# Rex module and agent skill

## Rex module

The Rex module adds a `rexamine` action. It opens rexamine in a pane beside the focused pane and sends comments to that pane. Run the action again to close the pane. Each tab has its own rexamine pane, so the action never closes a pane in another tab.

Focus your agent's pane and press the key to review the local changes. To review a pull request, run:

```sh
rex do rexamine pr=https://github.com/owner/repo/pull/123
```

The pane shows the checkout that the agent started in. When the agent works in a worktree, give the directory of that worktree to show its changes:

```sh
rex do rexamine dir=/path/to/worktree
```

The agent skill tells the agent to do this when it moves to another worktree. You can also ask the agent to do it.

To change the options, edit the `setup` call in `~/.config/rex/init.lua` and run `rex config reload`:

```lua
require("rexamine").setup{
  bin = "/path/to/rexamine",  -- Only when rexamine is not on PATH. Rex runs this path on the host of the session.
  key = "ctrl+alt+cmd+d",    -- The key that toggles the pane.
  ratio = 0.55,              -- The share of the width that the agent pane keeps.
}
```

### The pane says "command not found"

Rex starts rexamine from a non-interactive shell, which does not read `~/.zshrc`. See [Put rexamine on the PATH that Rex uses](install.md#put-rexamine-on-the-path-that-rex-uses). You can also set `bin` to the full path, but Rex runs that path on the host of the session, so it fails on a host where the path does not exist.

## Agent skill

The [`rexamine` skill](../skills/rexamine/SKILL.md) teaches an agent to open the pane and to read the comments that rexamine sends. Ask the agent to open a pull request in rexamine. In Claude Code, you can also run `/rexamine <pr>`.

Comments work with every agent without the skill, because rexamine pastes them into the pane as plain text.

`rexamine setup` installs the skill for each agent whose configuration directory exists. Agents that share a skills directory get one copy.

| Agent          | Skill directory                                   |
| -------------- | ------------------------------------------------- |
| Claude Code    | `~/.claude/skills` (or `$CLAUDE_CONFIG_DIR/skills`) |
| Codex          | `~/.agents/skills`                                |
| Pi             | `~/.agents/skills`                                |
| OpenCode       | `~/.config/opencode/skills`                       |
| Gemini CLI     | `~/.gemini/skills`                                |
| GitHub Copilot | `~/.copilot/skills`                               |
| Cursor         | `~/.cursor/skills`                                |
| Amp            | `~/.config/agents/skills`                         |
| Droid          | `~/.factory/skills`                               |
| Goose          | `~/.config/goose/skills`                          |
| Crush          | `~/.config/crush/skills`                          |

When you install another agent later, run `rexamine setup` again. See [Installation and setup](install.md).

## When Rex changes

> [!IMPORTANT]
> Rex is in beta, and an update can change the parts of Rex that rexamine uses. rexamine keeps working as a diff viewer in that case, but the pane or the comments can stop working until rexamine is updated.

rexamine uses these parts of Rex:

| Part                      | Used for                                                        |
| ------------------------- | --------------------------------------------------------------- |
| `rex api call block.event` | Paste a comment into the agent's pane.                         |
| `rex send-key` and `rex focus` | Submit a comment with `ctrl+s`, and focus the agent's pane. |
| The Lua configuration API | The `rexamine` action and its key: `rex.action`, `rex.bind`, `rex.session.new_split`, `rex.session.view`, `rex.block.call`, and `rex.call`. |
| `rex do rexamine`          | Open the pane from the agent skill.                             |
| `rex config reload`       | Load the module after `rexamine setup`.                          |

If something stops working after a Rex update:

1. **Comments do not reach the agent.** rexamine copies a comment to the clipboard when the paste fails, and shows the error at the bottom of the pane. Paste the comment into the agent yourself.
2. **The key or the action does nothing.** The lines that `rexamine setup` adds to `init.lua` catch an error in the rexamine module, so the rest of your configuration still loads. If `rex do rexamine` reports that the action does not exist, the module did not load. Update rexamine and run `rexamine setup` again.
3. **The agent cannot open the pane.** The agent skill tells the agent to read `rex skill show`, which describes the commands of the installed Rex version, and to open the pane with them. You can do the same by hand:

   ```sh
   rex split --split=right -- rexamine --agent "$REX_BLOCK"
   ```

4. **Report it.** Open an [issue](https://github.com/segersniels/rexamine/issues/new/choose) with the output of `rex version`, the error, and the output of `rex api describe block.event` if the comments fail.
