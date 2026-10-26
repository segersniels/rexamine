--- Rex integration for rexdiff.
---
--- Adds a "rexdiff" action that opens rexdiff in a pane beside the focused
--- pane, or closes it when it is already open. Comments written in rexdiff
--- are pasted into the pane it opened beside, such as a Claude Code or Codex session.
---
--- Usage in ~/.config/rex/init.lua:
---
---   require("rexdiff").setup{ key = "ctrl+alt+cmd+d" }
---
--- Then press the key for the local changes, or run the action with a pull
--- request: rex do rexdiff pr=https://github.com/owner/repo/pull/123
--- Give a directory to show the local changes of another checkout, such as a
--- worktree that an agent works in: rex do rexdiff dir=/path/to/worktree
local M = {}

--- Labels the rexdiff pane, so the action can find it again.
local LABEL = "rexdiff"

--- Finds the open rexdiff pane in the tab of the anchor pane, or in the shown
--- tab without an anchor. Each tab keeps its own pane, so that agents in
--- different tabs do not close each other's pane.
local function find_pane(session_id, anchor_id)
  local view = rex.session.view{ session_id = session_id }
  for _, window in ipairs(view.windows or {}) do
    local pane, has_anchor = nil, false
    for _, layer in ipairs(window.layers or {}) do
      for _, block in ipairs(layer.blocks or {}) do
        if block.label == LABEL then pane = block.block_id end
        if block.block_id == anchor_id then has_anchor = true end
      end
    end

    if has_anchor or (not anchor_id and window.window_id == view.active_window_id) then return pane end
  end
end

--- Gets the working directory of the program in a terminal pane, such as an
--- agent. It falls back to the directory the shell last reported.
local function pane_dir(block_id)
  local terminal = "com.superlogical.terminal"

  local process = rex.block.call(terminal, "process", { block_id = block_id })
  local program = process and (process.foreground or process.child)
  if program and program.cwd then return program.cwd end

  local reported = rex.block.call(terminal, "pwd", { block_id = block_id })
  if reported and reported.pwd then return (reported.pwd:gsub("^file://[^/]*", "")) end

  return os.getenv("HOME")
end

--- Registers the rexdiff action.
---
--- Options:
---   bin    Path to the rexdiff binary. Rex starts it from a non-interactive
---          shell, so set a full path when the binary is not on that PATH.
---          Defaults to "rexdiff".
---   key    A key that toggles the pane, such as "ctrl+alt+cmd+d".
---   ratio  The share of the width that the focused pane keeps. Defaults to 0.55.
function M.setup(opts)
  opts = opts or {}
  local bin = opts.bin or "rexdiff"
  local ratio = opts.ratio or 0.55

  rex.action{
    name        = "rexdiff",
    title       = "Toggle Diff Review",
    description = "Review the local changes, or a pull request, beside the focused pane.",
    keywords    = "diff review pull request pr",
    args        = { pr = "string?", dir = "string?" },
    run         = function(ctx, args)
      local open = find_pane(ctx.session_id, ctx.block_id)
      if open then rex.call("block.close", { session_id = ctx.session_id, block_id = open }) end

      -- Without a pull request or a directory the action toggles, so a second key press only closes the pane.
      if open and not args.pr and not args.dir then return { closed = open } end

      local command = { bin }
      if ctx.block_id then
        table.insert(command, "--agent")
        table.insert(command, ctx.block_id)
      end
      if args.pr then table.insert(command, args.pr) end

      local result, err = rex.session.new_split{
        anchor_id = ctx.block_id,
        direction = "horizontal",
        side      = "after",
        ratio     = ratio,
        focus     = ctx.origin == "key" or ctx.origin == "palette",
        layout    = { block = {
          flavor  = "com.superlogical.terminal.shell",
          label   = LABEL,
          options = { command = command, cwd = args.dir or pane_dir(ctx.block_id) },
        } },
      }
      if err ~= nil then error(err, 0) end
      return result
    end,
  }

  if opts.key then rex.bind(opts.key, "rexdiff") end
end

return M
