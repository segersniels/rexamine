# Rex module and agent skill

## Rex module

The Rex module adds a `rexdiff` action. It opens rexdiff in a pane beside the focused pane and sends comments to that pane. Run the action again to close the pane.

Focus your agent's pane and press the key to review the local changes. To review a pull request, run:

```sh
rex do rexdiff pr=https://github.com/owner/repo/pull/123
```

To change the options, edit the `setup` call in `~/.config/rex/init.lua` and run `rex config reload`:

```lua
require("rexdiff").setup{
  bin = "/path/to/rexdiff",  -- Rex starts rexdiff with this path.
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
