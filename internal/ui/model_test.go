package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexdiff/internal/agent"
	"github.com/segersniels/rexdiff/internal/diff"
	"github.com/segersniels/rexdiff/internal/github"
	"github.com/segersniels/rexdiff/internal/source"
	"github.com/segersniels/rexdiff/internal/update"
)

const testDiff = `diff --git a/src/settings-mapper.ts b/src/settings-mapper.ts
--- a/src/settings-mapper.ts
+++ b/src/settings-mapper.ts
@@ -1,4 +1,5 @@ import {
 import {
   type Language,
-  META_TEXT_MAX_LENGTH,
+  matchLanguage,
+  truncate, // a long trailing comment that has to wrap on a narrow screen to fit
 } from "@tallyforms/lib";
`

// fakeSource returns a fixed review.
type fakeSource struct{ review source.Review }

func (f fakeSource) Load() (source.Review, error) { return f.review, nil }

// fakeSender records what it is asked to send.
type fakeSender struct{ sent []string }

func (f *fakeSender) Send(text string) error {
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeSender) Target() string { return "agent" }

func testReview() source.Review {
	return source.Review{
		Context: "owner/repo #7",
		Title:   "Tidy imports",
		Base:    "main",
		Head:    "feature/a-branch-name-that-is-long-enough-to-wrap-on-a-narrow-screen",
		Diff:    testDiff,
		PullRequest: &github.PullRequest{
			Number: 7, Title: "Tidy imports", State: "OPEN", Author: github.Author{Login: "someone"},
			ReviewRequests: []github.ReviewRequest{{Login: "a-reviewer-with-a-login-that-is-too-long-for-a-narrow-pane"}, {Name: "core-team"}},
			LatestReviews:  []github.Review{{Author: github.Author{Login: "approver"}, State: "APPROVED"}},
			Labels:         []github.Label{{Name: "bug", Color: "d73a4a"}, {Name: "a-label-name-that-is-too-long-for-a-narrow-pane", Color: "fbca04"}, {Name: "no-color"}},
			Body:           "Removes an unused import. See https://example.com/a/very/long/link/that/does/not/fit.",
		},
	}
}

// newLoadedModel creates a model that has loaded the review on a screen of the size.
func newLoadedModel(t *testing.T, sender *fakeSender, width, height int) Model {
	t.Helper()
	review := testReview()

	var model tea.Model = New(fakeSource{review}, sender, nil, "dark")
	model, _ = model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})
	return model.(Model)
}

// press sends keys to the model, one message per key, and returns the
// command of the last key.
func press(m Model, keys ...string) (Model, tea.Cmd) {
	var model tea.Model = m
	var cmd tea.Cmd
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "shift+tab":
			msg = tea.KeyMsg{Type: tea.KeyShiftTab}
		}
		model, cmd = model.Update(msg)
	}
	return model.(Model), cmd
}

func TestViewFitsNarrowScreens(t *testing.T) {
	for _, width := range []int{24, 40, 60, 100} {
		m := newLoadedModel(t, &fakeSender{}, width, 200)
		browsing := m.View()

		m, _ = press(m, "]", "j", "j", "j", "c")
		if m.mode != composing {
			t.Fatalf("width %d: comment box did not open", width)
		}

		for _, view := range []string{browsing, m.View()} {
			for i, line := range strings.Split(view, "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Errorf("width %d: line %d is %d columns wide: %q", width, i+1, got, ansi.Strip(line))
				}
			}
		}
	}
}

func TestCommentOnSelectedLinesSendsThemToTheAgent(t *testing.T) {
	sender := &fakeSender{}
	m := newLoadedModel(t, sender, 50, 60)

	// Go to the file, then down past its title and hunk header to "type Language".
	m, _ = press(m, "]", "j", "j", "j")
	if r := m.rows[m.cursor]; r.kind != codeRow || !strings.Contains(r.text, "type Language") {
		t.Fatalf("cursor is on %q, want the type Language line", ansi.Strip(r.text))
	}

	m, _ = press(m, "v", "j", "j", "c")
	if m.mode != composing {
		t.Fatal("composer did not open")
	}

	m, _ = press(m, "W", "h", "y", "?")
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter did not send the comment")
	}
	cmd()

	want := "src/settings-mapper.ts L2-L3:\n```diff\n   type Language,\n-  META_TEXT_MAX_LENGTH,\n+  matchLanguage,\n```\nWhy?\n\n"
	if len(sender.sent) != 1 || sender.sent[0] != want {
		t.Errorf("sent %q, want %q", sender.sent, want)
	}
	if m.mode != browsing || m.anchor != -1 {
		t.Error("sending did not close the composer and clear the selection")
	}
}

func TestCommentSendsAWrappedLineOnce(t *testing.T) {
	sender := &fakeSender{}
	m := newLoadedModel(t, sender, 40, 60)

	// The last line wraps over several rows. Select from its last row upward.
	m, _ = press(m, "G")
	m, _ = press(m, "v", "k", "c")
	_, cmd := press(m, "enter")
	cmd()

	if len(sender.sent) != 1 || strings.Count(sender.sent[0], "truncate") != 1 {
		t.Errorf("sent %q, want the wrapped line once", sender.sent)
	}
}

func TestCommentOutsideTheDiffDoesNotOpenTheComposer(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 40)

	m, _ = press(m, "c")
	if m.mode == composing {
		t.Error("composer opened on the pull request title")
	}
}

func TestResizeKeepsTheCursorOnTheSameLine(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 80, 40)
	m, _ = press(m, "]", "j", "j", "j")
	before, _ := m.lineKeyAt(m.cursor)

	model, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 40})
	m = model.(Model)

	if after, ok := m.lineKeyAt(m.cursor); !ok || after != before {
		t.Errorf("cursor moved from %+v to %+v", before, after)
	}
}

// mouse sends a left-button event at a screen position.
func mouse(m Model, action tea.MouseAction, x, y int) Model {
	model, _ := m.Update(tea.MouseMsg{X: x, Y: y, Action: action, Button: tea.MouseButtonLeft})
	return model.(Model)
}

// screenY finds the screen line of the first page row whose text contains want.
func screenY(t *testing.T, m Model, want string) int {
	t.Helper()
	for y, line := range m.screenLines() {
		if line.row >= 0 && strings.Contains(ansi.Strip(m.rows[line.row].text), want) {
			return y + 1
		}
	}
	t.Fatalf("no row on the screen contains %q", want)
	return 0
}

func TestMouseDragOpensTheCommentBoxBelowTheSelection(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]")

	from, to := screenY(t, m, "type Language"), screenY(t, m, "matchLanguage")
	m = mouse(m, tea.MouseActionPress, 10, from)
	m = mouse(m, tea.MouseActionMotion, 10, to)
	m = mouse(m, tea.MouseActionRelease, 10, to)

	if m.mode != composing {
		t.Fatal("releasing a drag did not open the comment box")
	}
	if got := m.snippet.Location(); got != "L2-L3" {
		t.Errorf("comment box is for %s, want L2-L3", got)
	}

	// The box starts on the line below the last selected line.
	lines := m.screenLines()
	if lines[to].row != -1 || !strings.Contains(ansi.Strip(lines[to].text), "settings-mapper.ts L2-L3") {
		t.Errorf("line below the selection is %q, want the comment box title", ansi.Strip(lines[to].text))
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if got := ansi.StringWidth(line); got > 60 {
			t.Errorf("line %d is %d columns wide: %q", i+1, got, ansi.Strip(line))
		}
	}
}

func TestClickOnALineNumberOpensTheCommentBox(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]")

	y := screenY(t, m, "type Language")
	m = mouse(m, tea.MouseActionPress, 1, y)
	m = mouse(m, tea.MouseActionRelease, 1, y)

	if m.mode != composing || m.snippet.Location() != "L2" {
		t.Errorf("mode %v, location %q; want the comment box for L2", m.mode, m.snippet.Location())
	}
}

func TestClickOnCodeOnlyMovesTheCursor(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]")

	y := screenY(t, m, "type Language")
	m = mouse(m, tea.MouseActionPress, 20, y)
	m = mouse(m, tea.MouseActionRelease, 20, y)

	if m.mode == composing || m.anchor != -1 {
		t.Error("a click on the code opened the comment box")
	}
	if !strings.Contains(m.rows[m.cursor].text, "type Language") {
		t.Errorf("cursor is on %q, want the clicked line", ansi.Strip(m.rows[m.cursor].text))
	}
}

// hasCodeRows reports whether the page shows any diff line.
func hasCodeRows(m Model) bool {
	for _, r := range m.rows {
		if r.kind == codeRow {
			return true
		}
	}
	return false
}

func TestCollapseKeyHidesAndShowsTheFile(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)

	// Collapse from a line inside the file.
	m, _ = press(m, "]", "j", "j", "j", "z")
	if hasCodeRows(m) {
		t.Fatal("collapsed file still shows its lines")
	}
	if r := m.rows[m.cursor]; r.kind != fileRow || !strings.Contains(ansi.Strip(r.text), "▸ src/settings-mapper.ts") {
		t.Errorf("cursor is on %q, want the collapsed file title", ansi.Strip(r.text))
	}

	m, _ = press(m, "z")
	if !hasCodeRows(m) {
		t.Error("expanded file does not show its lines")
	}
}

func TestClickOnAFileTitleCollapsesTheFile(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)

	y := screenY(t, m, "settings-mapper.ts")
	m = mouse(m, tea.MouseActionPress, 10, y)
	m = mouse(m, tea.MouseActionRelease, 10, y)

	if hasCodeRows(m) || m.mode == composing {
		t.Error("a click on the file title did not collapse the file")
	}
}

func TestCollapsedFileStaysCollapsedAfterAReload(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]", "z")

	review := testReview()
	review.Diff += " // an edit that changes the diff\n"
	model, _ := m.Update(loadedMsg{gen: 1, review: review})

	if hasCodeRows(model.(Model)) {
		t.Error("the file expanded after a reload")
	}
}

func TestUpdateNoticeWrapsInNarrowScreens(t *testing.T) {
	notice := update.Notice{Version: "9.9.9"}

	for _, width := range []int{24, 40, 200} {
		var model tea.Model = newLoadedModel(t, &fakeSender{}, width, 60)
		model, _ = model.Update(notice)
		view := model.View()

		if !strings.Contains(ansi.Strip(view), "rexdiff 9.9.9") {
			t.Errorf("width %d: view does not show the notice", width)
		}
		for i, line := range strings.Split(view, "\n") {
			if got := ansi.StringWidth(line); got > width {
				t.Errorf("width %d: line %d is %d columns wide: %q", width, i+1, got, ansi.Strip(line))
			}
		}
		if width == 200 && !strings.Contains(ansi.Strip(view), "`rexdiff update`") {
			t.Errorf("view does not show the whole update command:\n%s", ansi.Strip(view))
		}
	}
}

func TestTurnWithoutEditsKeepsThePreviousTurnStart(t *testing.T) {
	var model tea.Model = New(source.Local{}, &fakeSender{}, make(chan struct{}), "dark")

	// Each snapshot is the working tree at the start of a turn. The second turn changes nothing.
	for _, tree := range []string{"first", "second", "second"} {
		model, _ = model.Update(snapshotMsg{tree: tree})
	}

	local := model.(Model).source.(source.Local)
	if local.TurnStart != "second" || local.PreviousTurnStart != "first" {
		t.Errorf("got turn start %q and previous %q, want second and first", local.TurnStart, local.PreviousTurnStart)
	}
}

func TestShiftTabSwitchesAPullRequestCommentToGitHub(t *testing.T) {
	sender := &fakeSender{}
	m := newLoadedModel(t, sender, 80, 60)

	m, _ = press(m, "]", "j", "j", "j", "c", "shift+tab")
	if m.destination != toPullRequest {
		t.Fatal("shift+tab did not switch the comment to GitHub")
	}
	if footer := ansi.Strip(m.footerView()); !strings.Contains(footer, "comment on GitHub") {
		t.Errorf("footer %q does not say that the comment goes to GitHub", footer)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "╰─ comment (shift+tab to cycle) ─") {
		t.Errorf("the comment box border does not show the destination and the key:\n%s", view)
	}

	// An empty comment stays in the composer instead of going to GitHub or the agent.
	m, _ = press(m, "enter")
	if m.mode != composing || len(sender.sent) != 0 {
		t.Error("enter sent an empty GitHub comment")
	}

	m, _ = press(m, "shift+tab")
	if m.destination != toAgent {
		t.Error("shift+tab did not switch the comment back to the agent")
	}
}

func TestShiftTabDoesNothingForLocalChanges(t *testing.T) {
	review := testReview()
	review.PullRequest = nil

	var model tea.Model = New(source.Local{}, &fakeSender{}, nil, "dark")
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	m, _ := press(model.(Model), "]", "j", "j", "j", "c", "shift+tab")
	if m.destination != toAgent {
		t.Error("shift+tab switched a local comment to GitHub")
	}
}

func TestReviewCommentUsesTheSideOfEachLine(t *testing.T) {
	snippet := agent.Snippet{Path: "a.go", Lines: []diff.Line{
		{Kind: diff.Removed, OldLine: 3},
		{Kind: diff.Added, NewLine: 3},
		{Kind: diff.Context, OldLine: 4, NewLine: 4},
	}}

	got := reviewComment(snippet, "Why?")
	want := github.ReviewComment{
		Path:  "a.go",
		Start: github.CommentLine{Number: 3, Side: github.Left},
		End:   github.CommentLine{Number: 4, Side: github.Right},
		Body:  "Why?",
	}
	if got != want {
		t.Errorf("reviewComment = %+v, want %+v", got, want)
	}
}

func TestPullRequestWithoutAnAgentOnlyCommentsOnGitHub(t *testing.T) {
	review := testReview()

	var model tea.Model = New(fakeSource{review}, agent.Clipboard{}, nil, "dark")
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	m, _ := press(model.(Model), "]", "j", "j", "j", "c", "shift+tab")
	if m.destination != toPullRequest {
		t.Error("the comment does not go to GitHub")
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "╰─ comment ─") {
		t.Errorf("the comment box border does not say comment without a hint:\n%s", view)
	}
}

func TestWorktreePickerSwitchesToTheChosenBranch(t *testing.T) {
	src := source.Local{Dir: "/repo", TurnStart: "old", PreviousTurnStart: "older"}
	var model tea.Model = New(src, &fakeSender{}, nil, "dark")
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	model, _ = model.Update(worktreesMsg{worktrees: []source.Worktree{
		{Path: "/repo", Branch: "main", Main: true, Current: true},
		{Path: "/repo-topic", Branch: "feat/a-branch-name-that-does-not-fit-in-the-pane"},
		{Path: "/repo-detached", Head: "0123456789abcdef"},
	}})

	m := model.(Model)
	if m.mode != picking {
		t.Fatal("picker did not open")
	}

	// Rows name the branch, not the directory, and the tags say which checkout is which.
	view := ansi.Strip(m.View())
	for _, want := range []string{"main", "current · main", "feat/a-branch", "0123456", "detached"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "/repo-topic") {
		t.Errorf("picker shows a directory:\n%s", view)
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if got := ansi.StringWidth(line); got > 40 {
			t.Errorf("line %d is %d columns wide: %q", i+1, got, ansi.Strip(line))
		}
	}

	m, cmd := press(m, "j", "enter")
	local := m.source.(source.Local)
	if m.mode != browsing || local.Dir != "/repo-topic" || cmd == nil {
		t.Errorf("got mode %d and dir %q, want the topic checkout to load", m.mode, local.Dir)
	}

	// The turn snapshots belong to the old checkout.
	if local.TurnStart != "" || local.PreviousTurnStart != "" {
		t.Errorf("turn snapshots were kept: %q, %q", local.TurnStart, local.PreviousTurnStart)
	}
}

func TestWorktreePickerWithOneCheckoutStaysClosed(t *testing.T) {
	var model tea.Model = New(source.Local{}, &fakeSender{}, nil, "dark")
	model, _ = model.Update(worktreesMsg{worktrees: []source.Worktree{{Path: "/repo", Branch: "main", Current: true}}})

	if m := model.(Model); m.mode == picking || !strings.Contains(m.status, "no other worktrees") {
		t.Errorf("got mode %d and status %q, want a status and no picker", m.mode, m.status)
	}
}

func TestReloadWhileHiddenDoesNotCrashOnReturn(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 80, 40)
	m, _ = press(m, "]", "j", "j", "j", "j")

	// The pane is hidden and gets a size of zero. Then the diff shrinks to one file.
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	review := testReview()
	review.Diff = "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if view := ansi.Strip(model.(Model).View()); !strings.Contains(view, "x") {
		t.Errorf("page does not show the new diff:\n%s", view)
	}
}

func TestFocusReloadsAPullRequestAfterTheCooldown(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 80, 40)

	var model tea.Model = m
	model, cmd := model.Update(tea.FocusMsg{})
	if cmd != nil {
		t.Fatal("focus right after the first load starts another load")
	}

	m = model.(Model)
	m.loadStarted = time.Now().Add(-focusCooldown)
	model, cmd = m.Update(tea.FocusMsg{})
	if cmd == nil || model.(Model).loadGen != 2 {
		t.Fatal("focus after the cooldown does not reload the pull request")
	}
}

func TestReviewerChipsShowEachReviewerOnce(t *testing.T) {
	pr := github.PullRequest{
		Author: github.Author{Login: "author"},
		ReviewRequests: []github.ReviewRequest{
			{Login: "approver"},
			{Login: "waiting"},
			{Name: "core-team"},
		},
		LatestReviews: []github.Review{
			{Author: github.Author{Login: "approver"}, State: "APPROVED"},
			{Author: github.Author{Login: "blocker"}, State: "CHANGES_REQUESTED"},
			{Author: github.Author{Login: "bot"}, State: "COMMENTED"},
			{Author: github.Author{Login: "author"}, State: "COMMENTED"},
			{Author: github.Author{Login: "dismissed"}, State: "DISMISSED"},
		},
	}

	var got []string
	for _, chip := range reviewerChips(pr) {
		got = append(got, ansi.Strip(chip))
	}

	want := []string{"✓ approver", "✗ blocker", "○ bot", "● waiting", "● core-team"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("reviewerChips() = %q, want %q", got, want)
	}
}
