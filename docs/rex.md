# Rex module and agent skill

## Rex module

The Rex module adds a `rexdiff` action. It opens rexdiff in a pane beside the focused pane and sends comments to that pane. Run the action again to close the pane. Each tab has its own rexdiff pane, so the action never closes a pane in another tab.

Focus your agent's pane and press the key to review the local changes. To review a pull request, run:

```sh
rex do rexdiff pr=https://github.com/owner/repo/pull/123
```

The pane shows the checkout that the agent started in. When the agent works in a worktree, give the directory of that worktree to show its changes:

```sh
rex do rexdiff dir=/path/to/worktree
```

The agent skill tells the agent to do this when it moves to another worktree. You can also ask the agent to do it.

To change the options, edit the `setup` call in `~/.config/rex/init.lua` and run `rex config reload`:

```lua
require("rexdiff").setup{
  bin = "/path/to/rexdiff",  -- Only when rexdiff is not on PATH. Rex runs this path on the host of the session.
  key = "ctrl+alt+cmd+d",    -- The key that toggles the pane.
  ratio = 0.55,              -- The share of the width that the agent pane keeps.
}
```

## Agent skill

The [`rexdiff` skill](../skills/rexdiff/SKILL.md) teaches an agent to open the pane and to read the comments that rexdiff sends. Ask the agent to open a pull request in rexdiff. In Claude Code, you can also run `/rexdiff <pr>`.

Comments work with every agent without the skill, because rexdiff pastes them into the pane as plain text.

`rexdiff setup` installs the skill for each agent whose configuration directory exists. Agents that share a skills directory get one copy.

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

When you install another agent later, run `rexdiff setup` again. See [Installation and setup](install.md).

## When Rex changes

> [!IMPORTANT]
> Rex is in beta, and an update can change the parts of Rex that rexdiff uses. rexdiff keeps working as a diff viewer in that case, but the pane or the comments can stop working until rexdiff is updated.

rexdiff uses these parts of Rex:

| Part                      | Used for                                                        |
| ------------------------- | --------------------------------------------------------------- |
| `rex api call block.event` | Paste a comment into the agent's pane.                         |
| `rex send-key` and `rex focus` | Submit a comment with `ctrl+s`, and focus the agent's pane. |
| The Lua configuration API | The `rexdiff` action and its key: `rex.action`, `rex.bind`, `rex.session.new_split`, `rex.session.view`, `rex.block.call`, and `rex.call`. |
| `rex do rexdiff`          | Open the pane from the agent skill.                             |
| `rex config reload`       | Load the module after `rexdiff setup`.                          |

If something stops working after a Rex update:

1. **Comments do not reach the agent.** rexdiff copies a comment to the clipboard when the paste fails, and shows the error at the bottom of the pane. Paste the comment into the agent yourself.
2. **The key or the action does nothing.** The lines that `rexdiff setup` adds to `init.lua` catch an error in the rexdiff module, so the rest of your configuration still loads. If `rex do rexdiff` reports that the action does not exist, the module did not load. Update rexdiff and run `rexdiff setup` again.
3. **The agent cannot open the pane.** The agent skill tells the agent to read `rex skill show`, which describes the commands of the installed Rex version, and to open the pane with them. You can do the same by hand:

   ```sh
   rex split --split=right -- rexdiff --agent "$REX_BLOCK"
   ```

4. **Report it.** Open an [issue](https://github.com/segersniels/rexdiff/issues/new/choose) with the output of `rex version`, the error, and the output of `rex api describe block.event` if the comments fail.
