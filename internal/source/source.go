// Package source loads the changes that rexamine shows: a GitHub pull request,
// or the local changes in a git repository.
package source

import (
	"fmt"

	"github.com/segersniels/rexamine/internal/github"
)

// Review is a diff to review and the details that describe it.
type Review struct {
	// Context names where the review comes from, such as "owner/repo #12"
	// for a pull request or the repository's name for local changes.
	Context string
	Title   string

	// Base is where the changes start from, and Head is what holds them,
	// such as a base branch and a feature branch.
	Base string
	Head string

	Diff string

	// Root is the directory that holds the reviewed files, so an editor can
	// open them. It is empty when no worktree has a pull request's branch.
	Root string

	// PullRequest is set when the review is a pull request.
	PullRequest *github.PullRequest

	// Files reads the full files that the diff compares, or is nil when there is no diff.
	Files FileReader
}

// FileReader reads the full text of the files in a review, so each hunk can
// get the syntax colors of its whole file.
type FileReader interface {
	// ReadFiles returns the text of oldPaths in the old version and of
	// newPaths in the new version, by path. It leaves out a file that it
	// cannot read, such as a file that a later change removed.
	ReadFiles(oldPaths, newPaths []string) (oldTexts, newTexts map[string]string)
}

// pullRequestFiles reads the files of a pull request through the GitHub API.
type pullRequestFiles struct {
	pr github.PullRequest
}

// ReadFiles reads the files where the pull request split from its base branch and at its head.
func (p pullRequestFiles) ReadFiles(oldPaths, newPaths []string) (map[string]string, map[string]string) {
	return github.ReadFiles(p.pr, oldPaths, newPaths)
}

// Source loads a review. rexamine calls Load again to reload it.
type Source interface {
	Load() (Review, error)
}

// PullRequest loads a GitHub pull request through gh.
type PullRequest struct {
	// Target is a pull request number or URL, as gh accepts it. An empty
	// target selects the pull request of the branch that is checked out in Dir.
	Target string

	// Dir is a directory in the repository, or empty for the working directory.
	Dir string
}

// Load fetches the pull request and its diff.
func (p PullRequest) Load() (Review, error) {
	pr, diff, err := github.Fetch(p.Target, p.Dir)
	if err != nil {
		return Review{}, err
	}

	// A failed search only means that the files cannot open in an editor, so the pull request still shows.
	var root string
	worktrees, _ := Worktrees(p.Dir)
	for _, w := range worktrees {
		if w.Branch == pr.HeadRefName {
			root = w.Path
			break
		}
	}

	return Review{
		Context:     fmt.Sprintf("%s #%d", github.Repository(pr.URL), pr.Number),
		Title:       pr.Title,
		Base:        pr.BaseRefName,
		Head:        pr.HeadRefName,
		Diff:        diff,
		Root:        root,
		PullRequest: &pr,
		Files:       pullRequestFiles{pr},
	}, nil
}
