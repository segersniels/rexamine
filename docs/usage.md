# Usage and keys

## What rexamine shows

- **Your local changes**: everything on the current branch compared to the default branch, only the uncommitted changes, or only what the agent changed in its last turn. Untracked files are included. The view updates as files change.
- **A GitHub pull request**: the description, review status, labels, and diff, through the [GitHub CLI](https://cli.github.com).

```sh
rexamine                  # the changes on the current branch
rexamine --uncommitted    # only the uncommitted changes
rexamine --base origin/feature-parent  # a stacked branch, compared to its parent branch
rexamine 123              # pull request 123 of the current repository
rexamine https://github.com/owner/repo/pull/123
```

Press `p` to switch between the local changes and the pull request of the branch. Each view keeps its own settings, such as the scope of the local changes.

- From a pull request, rexamine shows the local changes of the worktree that has the pull request's branch checked out. Without such a worktree, it shows the current checkout.
- After `w` switches to another worktree, `p` shows the pull request of that worktree's branch.

## Keys

| Key                      | Action                                                    |
| ------------------------ | --------------------------------------------------------- |
| `j` / `k`, arrows, wheel | Move down or up                                           |
| `ctrl+d` / `ctrl+u`      | Move half a page                                          |
| `f` / `b`, page keys     | Move a page                                               |
| `g` / `G`                | Go to the top or the end                                  |
| `]` / `[`                | Go to the next or previous file                           |
| `/`                      | Find a file by part of its path and go to it              |
| `z`, click a title       | Collapse or expand the file or the description            |
| `x`                      | Hide or show test files; rexamine remembers the choice     |
| `v`                      | Select lines                                              |
| `c`, `enter`             | Comment on the selection or the current line              |
| `shift+tab`, in the box  | Switch the comment between the agent and GitHub           |
| Mouse drag over lines    | Select the lines and comment on them                      |
| Click a line number      | Comment on that line                                      |
| `t`                      | Switch between branch, uncommitted, and last-turn changes |
| `w`                      | Show the changes of another worktree, by its branch       |
| `B`                      | Compare the branch changes with another branch            |
| `p`                      | Switch between the local changes and the pull request     |
| `r`                      | Reload                                                    |
| `o`                      | Open the pull request on GitHub                           |
| `?`                      | Show all keys                                             |
| `q`                      | Quit                                                      |

## Last agent turn

When rexamine runs in a pane beside an agent in Rex, it follows the status that the agent reports to Rex. When the agent starts to work, rexamine saves a snapshot of the working tree. The last-turn view compares that snapshot with the working tree, so it shows only the changes of the current or most recent turn.

- The header shows whether the agent works, and how long ago the turn started or ended.
- If the agent already works when rexamine opens, rexamine saves the snapshot right away. The view then shows the changes since rexamine opened, and the header names "rexamine opened" as the start.
- A turn without edits, such as an answer to a question, keeps the changes of the turn before it in view until the agent changes a file.
- The snapshot does not change your index or your staged changes.
- The agent must report its status to Rex, as Claude Code does.

## Comments

The comment box opens below the selected lines. In it, `enter` sends the comment, `alt+enter` starts a new line, and `esc` cancels. With an agent pane, `ctrl+s` sends the comment, submits it, and focuses the agent pane, so the agent answers at once. A click elsewhere on the page also cancels.

Where a comment goes depends on how rexamine runs. The bottom border of the comment box shows it:

| How rexamine runs                         | Local changes   | Pull request                                 |
| ---------------------------------------- | --------------- | -------------------------------------------- |
| In a pane beside an agent                | The agent       | The agent or GitHub. `shift+tab` switches.   |
| Without an agent pane, such as `rexamine` | The clipboard   | GitHub                                       |

- **The agent**: rexamine pastes the comment into the agent's pane as a real paste, so the agent waits for you to submit. `ctrl+s` submits the comment at once and focuses the agent pane.
- **GitHub**: rexamine posts the comment on the selected lines of the pull request at once, as your GitHub account. The box border changes color to show it. rexamine keeps the choice for the next comment.
- **The clipboard**: paste the comment into any agent yourself.

A comment arrives in the agent's prompt in this form:

````text
src/settings-mapper.ts L2-L5:
```diff
   type Language,
-  META_TEXT_MAX_LENGTH,
+  matchLanguage,
+  truncate,
```
Why do we still import truncate here?
````

The line numbers are in the new version of the file, or in the old version when the location ends in `(removed)`.
