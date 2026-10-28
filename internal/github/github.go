// Package github reads pull requests through the gh CLI, so prview uses the
// account and hosts that gh is already signed in to.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// prFields lists the pull request fields that rexamine asks gh for.
const prFields = "number,title,url,state,isDraft,author,updatedAt,baseRefName,headRefName,headRefOid,reviewDecision,reviewRequests,reviews,labels,body"

// Author is the GitHub account that wrote something.
type Author struct {
	Login string
}

// Label is a label on the pull request. Color is a hex color without "#".
type Label struct {
	Name  string
	Color string
}

// ReviewRequest is a user or team that is asked to review. A team has a Name
// and no Login.
type ReviewRequest struct {
	Login string
	Name  string
}

// Review is one review on the pull request. State is APPROVED,
// CHANGES_REQUESTED, COMMENTED, DISMISSED, or PENDING.
type Review struct {
	Author Author
	State  string
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
	HeadRefOid     string
	ReviewDecision string
	ReviewRequests []ReviewRequest
	Reviews        []Review
	Labels         []Label
	Body           string
}

// Fetch gets a pull request and its unified diff. The target is a number,
// URL, or branch, as gh accepts it. An empty target selects the pull request
// of the branch that is checked out in dir. An empty dir is the working
// directory.
func Fetch(target, dir string) (PullRequest, string, error) {
	type diffResult struct {
		diff string
		err  error
	}

	diffs := make(chan diffResult, 1)
	go func() {
		out, err := run(dir, withTarget([]string{"pr", "diff", "--color=never"}, target)...)
		diffs <- diffResult{string(out), err}
	}()

	var pr PullRequest
	out, err := run(dir, withTarget([]string{"pr", "view", "--json", prFields}, target)...)
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
	_, err := run("", "pr", "view", url, "--web")
	return err
}

// Side tells which version of the file a review comment line is in: "LEFT"
// for the base and "RIGHT" for the head.
type Side string

// The sides of a diff, as the GitHub API names them.
const (
	Left  Side = "LEFT"
	Right Side = "RIGHT"
)

// CommentLine is a line of a file in the pull request's diff.
type CommentLine struct {
	Number int
	Side   Side
}

// ReviewComment is a comment on a range of lines in one file of a pull
// request. Start and End are the same line for a comment on one line.
type ReviewComment struct {
	Path       string
	Start, End CommentLine
	Body       string
}

// PostReviewComment posts the comment on the pull request's head commit. GitHub
// publishes it at once, outside of a pending review.
func PostReviewComment(pr PullRequest, comment ReviewComment) error {
	parsed, err := url.Parse(pr.URL)
	if err != nil {
		return fmt.Errorf("read pull request URL: %w", err)
	}

	args := []string{
		"api", fmt.Sprintf("repos/%s/pulls/%d/comments", Repository(pr.URL), pr.Number),
		"--hostname", parsed.Host,
		"-f", "commit_id=" + pr.HeadRefOid,
		"-f", "path=" + comment.Path,
		"-f", "body=" + comment.Body,
		"-F", "line=" + strconv.Itoa(comment.End.Number),
		"-f", "side=" + string(comment.End.Side),
	}
	if comment.Start != comment.End {
		args = append(args,
			"-F", "start_line="+strconv.Itoa(comment.Start.Number),
			"-f", "start_side="+string(comment.Start.Side),
		)
	}

	_, err = run("", args...)
	return err
}

// maxParallelFetches limits how many gh processes fetch files at the same time.
const maxParallelFetches = 8

var (
	fileCacheMu sync.Mutex

	// fileCache keeps file texts by host, repository, commit, and path. A file
	// at a commit never changes, so a reload of the same pull request fetches
	// no file again.
	fileCache = map[string]string{}
)

// ReadFiles reads oldPaths at the commit where the pull request split from its
// base branch, and newPaths at its head commit, by path. gh pr diff compares
// the same two commits. It leaves out a file that it cannot fetch.
func ReadFiles(pr PullRequest, oldPaths, newPaths []string) (oldTexts, newTexts map[string]string) {
	parsed, err := url.Parse(pr.URL)
	if err != nil {
		return map[string]string{}, map[string]string{}
	}
	host, repo := parsed.Host, Repository(pr.URL)
	newTexts = readFilesAt(host, repo, pr.HeadRefOid, newPaths)
	if len(oldPaths) == 0 {
		return map[string]string{}, newTexts
	}

	mergeBase, err := run("", "api", fmt.Sprintf("repos/%s/compare/%s...%s?per_page=1", repo, pr.BaseRefName, pr.HeadRefOid),
		"--hostname", host, "--jq", ".merge_base_commit.sha")
	if err != nil {
		return map[string]string{}, newTexts
	}
	return readFilesAt(host, repo, strings.TrimSpace(string(mergeBase)), oldPaths), newTexts
}

// readFilesAt fetches the files at paths in a commit, a few at a time.
func readFilesAt(host, repo, commit string, paths []string) map[string]string {
	texts := map[string]string{}
	var textsMu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, maxParallelFetches)

	for _, path := range paths {
		key := host + "/" + repo + "@" + commit + ":" + path
		fileCacheMu.Lock()
		text, cached := fileCache[key]
		fileCacheMu.Unlock()
		if cached {
			textsMu.Lock()
			texts[path] = text
			textsMu.Unlock()
			continue
		}

		wg.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()

			escaped := strings.Split(path, "/")
			for i, segment := range escaped {
				escaped[i] = url.PathEscape(segment)
			}
			out, err := run("", "api", fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, strings.Join(escaped, "/"), commit),
				"--hostname", host, "-H", "Accept: application/vnd.github.raw")
			if err != nil {
				return
			}

			fileCacheMu.Lock()
			fileCache[key] = string(out)
			fileCacheMu.Unlock()
			textsMu.Lock()
			texts[path] = string(out)
			textsMu.Unlock()
		})
	}
	wg.Wait()
	return texts
}

// Repository reads "owner/repo" from a pull request URL, such as
// https://github.com/owner/repo/pull/12.
func Repository(pullRequestURL string) string {
	parsed, err := url.Parse(pullRequestURL)
	if err != nil {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

// withTarget adds the target to gh arguments when there is one.
func withTarget(args []string, target string) []string {
	if target == "" {
		return args
	}
	return append(args, target)
}

// run runs gh in dir and turns a failure into an error that holds gh's own
// message. An empty dir is the working directory.
func run(dir string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
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
