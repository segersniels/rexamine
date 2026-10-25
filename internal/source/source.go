// Package source loads the changes that rexdiff shows: a GitHub pull request,
// or the local changes in a git repository.
package source

import (
	"fmt"

	"github.com/segersniels/rexdiff/internal/github"
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

	// PullRequest is set when the review is a pull request.
	PullRequest *github.PullRequest
}

// Source loads a review. rexdiff calls Load again to reload it.
type Source interface {
	Load() (Review, error)
}

// PullRequest loads a GitHub pull request through gh.
type PullRequest struct {
	// Target is a pull request number or URL, as gh accepts it.
	Target string
}

// Load fetches the pull request and its diff.
func (p PullRequest) Load() (Review, error) {
	pr, diff, err := github.Fetch(p.Target)
	if err != nil {
		return Review{}, err
	}
	return Review{
		Context:     fmt.Sprintf("%s #%d", github.Repository(pr.URL), pr.Number),
		Title:       pr.Title,
		Base:        pr.BaseRefName,
		Head:        pr.HeadRefName,
		Diff:        diff,
		PullRequest: &pr,
	}, nil
}
