# Contributing to rexdiff

Thank you for helping to improve rexdiff. This guide explains how to report a problem, how to propose a change, and how a release works.

## Report a bug or ask for a feature

Open an [issue](https://github.com/segersniels/rexdiff/issues/new/choose) and pick the bug report or the feature request form. For a bug, include the output of `rexdiff --version` and the steps that cause the problem.

For a large change, open an issue before you write code. Then we can agree on the approach before you spend time on it.

## Set up the project

You need:

- [Go](https://go.dev/dl/) 1.24 or later.
- `git`.
- The [GitHub CLI](https://cli.github.com) (`gh`), to test pull request reviews.
- [Rex](https://www.superlogical.com/rex), to test the pane and sending comments. Without Rex, rexdiff copies comments to the clipboard.

Clone your fork and build:

```sh
git clone https://github.com/<your-user>/rexdiff.git
cd rexdiff
make install   # build and install to ~/.local/bin
make setup     # install, then add the Rex module and the agent skill
```

## Make a change

1. Create a branch from `main`.
2. Make your change. Keep it focused on one problem.
3. Add or update tests for the behavior that you change. The tests are next to the code, in `*_test.go` files.
4. Run the checks that CI runs:

   ```sh
   make check   # go vet, golangci-lint, and the tests with the race detector
   make fmt     # format the code, if the lint step reports formatting
   ```

5. Update `README.md` when you change a key, a flag, or other behavior that users see.

### Code style

- Follow the existing code. golangci-lint enforces formatting and common mistakes.
- Write a doc comment for every function and type. Start a function's comment with a verb that says what it does.
- Comment why the code does something when the code cannot say it. Do not restate what the code does.

### Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org): `type(scope): description`, for example `fix(ui): keep the cursor on the line after a reload`. Use a type such as `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, or `chore`.

## Open a pull request

1. Push your branch to your fork and open a pull request against `main`.
2. Describe what the pull request changes and why, and link the issue that it fixes.
3. CI must pass. For a first contribution, a maintainer approves the CI run before it starts.
4. A maintainer reviews the pull request and merges it. Only maintainers can merge.

## Release a version

Only maintainers can release. A release builds the binaries for macOS and Linux, publishes a GitHub release with them, and publishes the npm packages.

1. Make sure that `main` has the changes for the release and that CI passes.
2. Open **Actions**, select **Release**, and select **Run workflow** on `main`.
3. Enter the version, such as `0.2.0`, without a `v`. Prerelease versions are not supported.

The workflow runs the tests, pushes the tag `v<version>`, runs GoReleaser, and publishes to npm. If a step fails after the tag is pushed, fix the cause, delete the tag and any GitHub release that the run created, and run the workflow again. The npm step skips packages that are already published.

npm publishes with [trusted publishing](https://docs.npmjs.com/trusted-publishers), so the workflow needs no npm token. Each npm package (`@segersniels/rexdiff` and the four `@segersniels/rexdiff-<os>-<cpu>` packages) must list this repository and the `release.yml` workflow as a trusted publisher.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
