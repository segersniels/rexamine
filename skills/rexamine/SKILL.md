---
name: rexamine
description: Open a diff review pane beside this session in Rex, for the local changes or a GitHub pull request, and understand the review comments the user sends from it. Use when the user asks to show, open, or review a diff or pull request beside the session, or when a message contains rexamine comments.
---

# rexamine

rexamine shows a diff in a Rex pane beside this session. The user selects lines in it and writes comments, and rexamine pastes each comment into this session's prompt.

## Open the pane

```sh
rex do -s "$REX_SESSION" -b "$REX_BLOCK" rexamine pr=<PR URL or number>
```

- Leave out `pr=` to show the local changes of the current branch. The user can press `t` in the pane to switch to uncommitted changes only, or to the changes of your last turn. The user can press `p` to switch between the local changes and the pull request of the branch.
- The local changes compare with the default branch, such as `origin/main`. When the current branch is stacked on another branch, add `base=<parent branch>`, such as `base=origin/feature-parent`, so the pane shows only the changes of this branch. Do this without being asked when you know the parent branch, for example from the pull request base (`gh pr view --json baseRefName`) or from the branch that you created this branch from.
- Run the command again with `pr=` to show another pull request. Run it without `pr=`, `dir=`, or `base=` to close the pane.
- The pane shows the checkout that your process started in. When you start to work in another worktree or checkout, run the command with `dir="$PWD"` from that directory, so the pane shows your changes there. Do this without being asked.
- If `REX_SESSION` or `REX_BLOCK` is not set, tell the user that the pane only works inside Rex. They can still run `rexamine` in another terminal, and it copies comments on local changes to the clipboard and posts comments on a pull request to GitHub.
- Do not wait for the pane or read its output. The pane is for the user.
- If Rex does not know the `rexamine` action, tell the user to run `rexamine setup`.
- If the command fails in another way, Rex may have changed, because Rex is in beta. Run `rex skill show` to read the commands of the installed Rex version. Use them to open rexamine in a split beside this pane, with `--agent "$REX_BLOCK"` so comments come back here, and `--base <branch>` for a stacked branch. For example: `rex split -s "$REX_SESSION" -b "$REX_BLOCK" --split=right --cwd "$PWD" -- rexamine --agent "$REX_BLOCK" <PR>`. Then tell the user that rexamine may need an update.

## Comments from the pane

A comment arrives in the user's message in this form:

````text
path/to/file.ts L12-L15:
```diff
 unchanged line
-removed line
+added line
```
The user's question or remark.
````

- The path is relative to the repository root. The line numbers are in the new version of the file, or in the old version when the location ends in `(removed)`.
- The diff block holds only the lines the user selected. Read the file for more context before you answer.
- A message can hold several comments. Answer each one, and name its file and lines in your answer.
