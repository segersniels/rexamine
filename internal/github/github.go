// Package github reads pull requests through the gh CLI, so prview uses the
// account and hosts that gh is already signed in to.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// prFields lists the pull request fields that rexdiff asks gh for.
const prFields = "number,title,url,state,isDraft,author,updatedAt,baseRefName,headRefName,reviewDecision,labels,body"

// Author is the GitHub account that wrote something.
type Author struct {
	Login string
}

// Label is a label on the pull request. Color is a hex color without "#".
type Label struct {
	Name  string
	Color string
}

// PullRequest holds the parts of a pull request that prview shows.
type PullRequest struct {
	Number         int
	Title          string
	URL            string
	State          string
	IsDraft        bool
	Author         Author
	UpdatedAt      time.Time
	BaseRefName    string
	HeadRefName    string
	ReviewDecision string
	Labels         []Label
	Body           string
}

// Fetch gets a pull request and its unified diff. The target is a number,
// URL, or branch, as gh accepts it. An empty target selects the pull request
// of the current branch.
func Fetch(target string) (PullRequest, string, error) {
	type diffResult struct {
		diff string
		err  error
	}

	diffs := make(chan diffResult, 1)
	go func() {
		out, err := run(withTarget([]string{"pr", "diff", "--color=never"}, target)...)
		diffs <- diffResult{string(out), err}
	}()

	var pr PullRequest
	out, err := run(withTarget([]string{"pr", "view", "--json", prFields}, target)...)
	if err != nil {
		return pr, "", err
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return pr, "", fmt.Errorf("read gh output: %w", err)
	}

	result := <-diffs
	return pr, result.diff, result.err
}

// OpenInBrowser opens the pull request on GitHub.
func OpenInBrowser(url string) error {
	_, err := run("pr", "view", url, "--web")
	return err
}

// withTarget adds the target to gh arguments when there is one.
func withTarget(args []string, target string) []string {
	if target == "" {
		return args
	}
	return append(args, target)
}

// run runs gh and turns a failure into an error that holds gh's own message.
func run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:2], " "), message)
	}
	return out, nil
}
