# Usage and keys

## What rexdiff shows

- **Your local changes**: everything on the current branch compared to the default branch, or only the uncommitted changes. Untracked files are included. The view updates as files change.
- **A GitHub pull request**: the description, reviews, comments, and diff, through the [GitHub CLI](https://cli.github.com).

```sh
rexdiff                  # the changes on the current branch
rexdiff --uncommitted    # only the uncommitted changes
rexdiff 123              # pull request 123 of the current repository
rexdiff https://github.com/owner/repo/pull/123
```

## Keys

| Key                       | Action                                         |
| ------------------------- | ---------------------------------------------- |
| `j` / `k`, arrows, wheel  | Move down or up                                |
| `ctrl+d` / `ctrl+u`       | Move half a page                               |
| `f` / `b`, page keys      | Move a page                                    |
| `g` / `G`                 | Go to the top or the end                       |
| `]` / `[`                 | Go to the next or previous file                |
| `z`, click a file title   | Collapse or expand the file                    |
| `v`                       | Select lines                                   |
| `c`, `enter`              | Comment on the selection or the current line   |
| Mouse drag over lines     | Select the lines and comment on them           |
| Click a line number       | Comment on that line                           |
| `t`                       | Switch between branch and uncommitted changes  |
| `r`                       | Reload                                         |
| `o`                       | Open the pull request on GitHub                |
| `?`                       | Show all keys                                  |
| `q`                       | Quit                                           |

## Comments

The comment box opens below the selected lines. In it, `enter` sends the comment, `alt+enter` starts a new line, and `esc` cancels. A click elsewhere on the page also cancels.

Inside Rex, rexdiff pastes the comment into the agent's pane as a real paste, so the agent waits for you to submit. Outside Rex, rexdiff copies the comment to the clipboard.

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
