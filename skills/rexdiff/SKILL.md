---
name: rexdiff
description: Open a diff review pane beside this session in Rex, for the local changes or a GitHub pull request, and understand the review comments the user sends from it. Use when the user asks to show, open, or review a diff or pull request beside the session, or when a message contains rexdiff comments.
---

# rexdiff

rexdiff shows a diff in a Rex pane beside this session. The user selects lines in it and writes comments, and rexdiff pastes each comment into this session's prompt.

## Open the pane

```sh
rex do -s "$REX_SESSION" -b "$REX_BLOCK" rexdiff pr=<PR URL or number>
```

- Leave out `pr=` to show the local changes of the current branch. The user can press `t` in the pane to switch to uncommitted changes only, or to the changes of your last turn.
- Run the command again with `pr=` to show another pull request. Run it without `pr=` to close the pane.
- If `REX_SESSION` or `REX_BLOCK` is not set, tell the user that the pane only works inside Rex. They can still run `rexdiff` in another terminal, and it copies comments on local changes to the clipboard and posts comments on a pull request to GitHub.
- Do not wait for the pane or read its output. The pane is for the user.
- If Rex does not know the `rexdiff` action, tell the user to run `rexdiff setup`.
- If the command fails in another way, Rex may have changed, because Rex is in beta. Run `rex skill show` to read the commands of the installed Rex version. Use them to open rexdiff in a split beside this pane, with `--agent "$REX_BLOCK"` so comments come back here. For example: `rex split -s "$REX_SESSION" -b "$REX_BLOCK" --split=right -- rexdiff --agent "$REX_BLOCK" <PR>`. Then tell the user that rexdiff may need an update.

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
