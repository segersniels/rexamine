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

The [`rexdiff` skill](../skills/rexdiff/SKILL.md) teaches an agent to open the pane and to read the comments that rexdiff sends. In Claude Code, ask the agent to open a pull request in rexdiff, or run `/rexdiff <pr>`.

`rexdiff setup` installs the skill to `~/.claude/skills/rexdiff`. See [Installation and setup](install.md).
