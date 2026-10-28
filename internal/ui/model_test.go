package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexamine/internal/agent"
	"github.com/segersniels/rexamine/internal/config"
	"github.com/segersniels/rexamine/internal/diff"
	"github.com/segersniels/rexamine/internal/editor"
	"github.com/segersniels/rexamine/internal/github"
	"github.com/segersniels/rexamine/internal/source"
	"github.com/segersniels/rexamine/internal/update"
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

// fakeSender records what it is asked to send or submit.
type fakeSender struct{ sent, submitted []string }

func (f *fakeSender) Send(text string) error {
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeSender) Submit(text string) error {
	f.submitted = append(f.submitted, text)
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
			Reviews:        []github.Review{{Author: github.Author{Login: "approver"}, State: "APPROVED"}},
			Labels:         []github.Label{{Name: "bug", Color: "d73a4a"}, {Name: "a-label-name-that-is-too-long-for-a-narrow-pane", Color: "fbca04"}, {Name: "no-color"}},
			Body:           "Removes an unused import. See https://example.com/a/very/long/link/that/does/not/fit.",
		},
	}
}

// newLoadedModel creates a model that has loaded the review on a screen of the size.
func newLoadedModel(t *testing.T, sender *fakeSender, width, height int) Model {
	t.Helper()
	review := testReview()

	var model tea.Model = New(fakeSource{review}, sender, nil, "dark", config.Config{})
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
		msg := tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEsc}
		case "ctrl+s":
			msg = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
		case "shift+tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		}
		model, cmd = model.Update(msg)
	}
	return model.(Model), cmd
}

func TestViewFitsNarrowScreens(t *testing.T) {
	for _, width := range []int{24, 40, 60, 100} {
		m := newLoadedModel(t, &fakeSender{}, width, 200)
		browsing := m.View().Content

		m, _ = press(m, "]", "j", "j", "j", "c")
		if m.mode != composing {
			t.Fatalf("width %d: comment box did not open", width)
		}

		for _, view := range []string{browsing, m.View().Content} {
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
	if r := m.rows[m.cursor]; r.kind != codeRow || !strings.Contains(ansi.Strip(r.text), "type Language") {
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

func TestSendNowKeySubmitsTheCommentToTheAgent(t *testing.T) {
	sender := &fakeSender{}
	m := newLoadedModel(t, sender, 50, 60)

	m, _ = press(m, "]", "j", "j", "j", "c", "W", "h", "y", "?")
	m, cmd := press(m, "ctrl+s")
	if cmd == nil {
		t.Fatal("ctrl+s did not submit the comment")
	}
	cmd()

	want := "src/settings-mapper.ts L2:\n```diff\n   type Language,\n```\nWhy?\n\n"
	if len(sender.submitted) != 1 || sender.submitted[0] != want || len(sender.sent) != 0 {
		t.Errorf("submitted %q and sent %q, want only %q submitted", sender.submitted, sender.sent, want)
	}
	if m.mode != browsing {
		t.Error("submitting did not close the composer")
	}
}

func TestSendNowKeyIsOffWithoutAnAgentPane(t *testing.T) {
	var model tea.Model = New(source.Local{}, agent.Clipboard{}, nil, "dark", config.Config{})
	if model.(Model).composeKeys.SendNow.Enabled() {
		t.Error("ctrl+s is on, but the clipboard cannot submit a comment")
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

// mouse sends a mouse event to the model.
func mouse(m Model, msg tea.MouseMsg) Model {
	model, _ := m.Update(msg)
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
	m = mouse(m, tea.MouseClickMsg{X: 10, Y: from, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseMotionMsg{X: 10, Y: to, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseReleaseMsg{X: 10, Y: to, Button: tea.MouseLeft})

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
	for i, line := range strings.Split(m.View().Content, "\n") {
		if got := ansi.StringWidth(line); got > 60 {
			t.Errorf("line %d is %d columns wide: %q", i+1, got, ansi.Strip(line))
		}
	}
}

func TestClickOnALineNumberOpensTheCommentBox(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]")

	y := screenY(t, m, "type Language")
	m = mouse(m, tea.MouseClickMsg{X: 1, Y: y, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseReleaseMsg{X: 1, Y: y, Button: tea.MouseLeft})

	if m.mode != composing || m.snippet.Location() != "L2" {
		t.Errorf("mode %v, location %q; want the comment box for L2", m.mode, m.snippet.Location())
	}
}

func TestClickOnCodeOnlyMovesTheCursor(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	m, _ = press(m, "]")

	y := screenY(t, m, "type Language")
	m = mouse(m, tea.MouseClickMsg{X: 20, Y: y, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseReleaseMsg{X: 20, Y: y, Button: tea.MouseLeft})

	if m.mode == composing || m.anchor != -1 {
		t.Error("a click on the code opened the comment box")
	}
	if !strings.Contains(ansi.Strip(m.rows[m.cursor].text), "type Language") {
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
	m = mouse(m, tea.MouseClickMsg{X: 10, Y: y, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseReleaseMsg{X: 10, Y: y, Button: tea.MouseLeft})

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

// newLongDescriptionModel creates a loaded model whose pull request has a
// description that is too long to show in full.
func newLongDescriptionModel(t *testing.T) Model {
	t.Helper()
	review := testReview()
	review.PullRequest.Body = "First paragraph.\n\nSecond paragraph.\n\nThird paragraph.\n\nThe last paragraph."

	var model tea.Model = New(fakeSource{review}, &fakeSender{}, nil, "dark", config.Config{})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 60})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})
	return model.(Model)
}

// pageText joins the text of every page row without colors.
func pageText(m Model) string {
	var lines []string
	for _, r := range m.rows {
		lines = append(lines, ansi.Strip(r.text))
	}
	return strings.Join(lines, "\n")
}

func TestLongDescriptionStartsCollapsedAndExpandsWithTheKey(t *testing.T) {
	m := newLongDescriptionModel(t)
	if text := pageText(m); strings.Contains(text, "The last paragraph.") || !strings.Contains(text, "more lines") {
		t.Fatalf("description is not collapsed:\n%s", text)
	}

	for m.rows[m.cursor].kind != descriptionRow {
		m, _ = press(m, "j")
	}
	m, _ = press(m, "z")
	if text := pageText(m); !strings.Contains(text, "The last paragraph.") || strings.Contains(text, "more lines") {
		t.Fatalf("z on the heading did not expand the description:\n%s", text)
	}
	if r := m.rows[m.cursor]; r.kind != descriptionRow {
		t.Errorf("cursor is on %q, want the description heading", ansi.Strip(r.text))
	}

	m, _ = press(m, "z")
	if strings.Contains(pageText(m), "The last paragraph.") {
		t.Error("z on the heading did not collapse the description")
	}
}

func TestClickOnMoreLinesExpandsTheDescription(t *testing.T) {
	m := newLongDescriptionModel(t)

	y := screenY(t, m, "more lines")
	m = mouse(m, tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseLeft})
	m = mouse(m, tea.MouseReleaseMsg{X: 5, Y: y, Button: tea.MouseLeft})

	if !strings.Contains(pageText(m), "The last paragraph.") {
		t.Error("a click on the more lines row did not expand the description")
	}
}

func TestShortDescriptionShowsInFull(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 60)
	if text := pageText(m); !strings.Contains(text, "Removes an unused import.") || strings.Contains(text, "more lines") {
		t.Errorf("short description is not shown in full:\n%s", text)
	}
}

func TestUpdateNoticeWrapsInNarrowScreens(t *testing.T) {
	notice := update.Notice{Version: "9.9.9"}

	for _, width := range []int{24, 40, 200} {
		var model tea.Model = newLoadedModel(t, &fakeSender{}, width, 60)
		model, _ = model.Update(notice)
		view := model.View().Content

		if !strings.Contains(ansi.Strip(view), "rexamine 9.9.9") {
			t.Errorf("width %d: view does not show the notice", width)
		}
		for i, line := range strings.Split(view, "\n") {
			if got := ansi.StringWidth(line); got > width {
				t.Errorf("width %d: line %d is %d columns wide: %q", width, i+1, got, ansi.Strip(line))
			}
		}
		if width == 200 && !strings.Contains(ansi.Strip(view), "`rexamine update`") {
			t.Errorf("view does not show the whole update command:\n%s", ansi.Strip(view))
		}
	}
}

func TestTurnWithoutEditsKeepsThePreviousTurnStart(t *testing.T) {
	var model tea.Model = New(source.Local{}, &fakeSender{}, make(chan agent.TurnEvent), "dark", config.Config{})

	// Each snapshot is the working tree at the start of a turn. The second turn changes nothing.
	for _, tree := range []string{"first", "second", "second"} {
		model, _ = model.Update(snapshotMsg{tree: tree})
	}

	local := model.(Model).source.(source.Local)
	if local.TurnStart != "second" || local.PreviousTurnStart != "first" {
		t.Errorf("got turn start %q and previous %q, want second and first", local.TurnStart, local.PreviousTurnStart)
	}
}

func TestTurnEventsTrackTheAgentsTurn(t *testing.T) {
	turns := make(chan agent.TurnEvent)
	var model tea.Model = New(source.Local{}, &fakeSender{}, turns, "dark", config.Config{})

	// rexamine opened while the agent worked, so it saves the working tree from now.
	model, cmd := model.Update(turnMsg{agent.TurnJoined})
	local := model.(Model).source.(source.Local)
	if !local.JoinedTurn || local.TurnStartedAt.IsZero() || cmd == nil {
		t.Fatalf("got %+v, want a joined turn that saves a snapshot", local)
	}

	model, _ = model.Update(turnMsg{agent.TurnEnded})
	local = model.(Model).source.(source.Local)
	if local.TurnEndedAt.IsZero() {
		t.Error("the end of the turn was not saved")
	}

	model, _ = model.Update(turnMsg{agent.TurnStarted})
	local = model.(Model).source.(source.Local)
	if local.JoinedTurn || !local.TurnEndedAt.IsZero() {
		t.Errorf("got %+v, want a new turn that runs", local)
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
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "╰─ comment (shift+tab to cycle) ─") {
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

	var model tea.Model = New(source.Local{}, &fakeSender{}, nil, "dark", config.Config{})
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

	var model tea.Model = New(fakeSource{review}, agent.Clipboard{}, nil, "dark", config.Config{})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	m, _ := press(model.(Model), "]", "j", "j", "j", "c", "shift+tab")
	if m.destination != toPullRequest {
		t.Error("the comment does not go to GitHub")
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "╰─ comment ─") {
		t.Errorf("the comment box border does not say comment without a hint:\n%s", view)
	}
}

func TestWorktreePickerSwitchesToTheChosenBranch(t *testing.T) {
	src := source.Local{Dir: "/repo", TurnStart: "old", PreviousTurnStart: "older"}
	var model tea.Model = New(src, &fakeSender{}, nil, "dark", config.Config{})
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
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"main", "current · main", "feat/a-branch", "0123456", "detached"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker does not show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "/repo-topic") {
		t.Errorf("picker shows a directory:\n%s", view)
	}
	for i, line := range strings.Split(m.View().Content, "\n") {
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

	// The pull request follows the checkout, so it is the one of the topic branch.
	m, _ = press(m, "p")
	if pr, ok := m.source.(source.PullRequest); !ok || pr.Dir != "/repo-topic" || pr.Target != "" {
		t.Errorf("got source %#v, want the pull request of the topic checkout", m.source)
	}
}

func TestWorktreePickerWithOneCheckoutStaysClosed(t *testing.T) {
	var model tea.Model = New(source.Local{}, &fakeSender{}, nil, "dark", config.Config{})
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
	if view := ansi.Strip(model.(Model).View().Content); !strings.Contains(view, "x") {
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
		Reviews: []github.Review{
			{Author: github.Author{Login: "approver"}, State: "APPROVED"},
			{Author: github.Author{Login: "blocker"}, State: "COMMENTED"},
			{Author: github.Author{Login: "blocker"}, State: "CHANGES_REQUESTED"},
			{Author: github.Author{Login: "approver"}, State: "COMMENTED"},
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

func TestSwitchKeyMovesBetweenThePullRequestAndTheLocalChanges(t *testing.T) {
	var model tea.Model = New(source.PullRequest{Target: "12"}, &fakeSender{}, make(chan agent.TurnEvent), "dark", config.Config{})

	m, cmd := press(model.(Model), "p")
	local, ok := m.source.(source.Local)
	if !ok || cmd == nil || !m.loading {
		t.Fatalf("got source %T, want the local changes to load", m.source)
	}
	if !m.keys.Toggle.Enabled() || m.keys.Open.Enabled() || m.composeKeys.Cycle.Enabled() {
		t.Error("the keys still belong to the pull request")
	}

	// A turn that starts while the pull request shows still saves its snapshot for the local changes.
	local.Scope = source.Uncommitted
	m.source = local
	m, _ = press(m, "p")
	model, _ = m.Update(snapshotMsg{tree: "turn"})
	m = model.(Model)
	if pr, ok := m.source.(source.PullRequest); !ok || pr.Target != "12" {
		t.Fatalf("got source %#v, want pull request 12", m.source)
	}

	m, _ = press(m, "p")
	local = m.source.(source.Local)
	if local.Scope != source.Uncommitted || local.TurnStart != "turn" {
		t.Errorf("got scope %v and turn start %q, want the local settings kept", local.Scope, local.TurnStart)
	}
}

func TestSwitchKeyShowsThePullRequestOfTheBranch(t *testing.T) {
	m, _ := press(New(source.Local{}, &fakeSender{}, nil, "dark", config.Config{}), "p")
	if pr, ok := m.source.(source.PullRequest); !ok || pr.Target != "" {
		t.Errorf("got source %#v, want the pull request of the current branch", m.source)
	}
	if !m.keys.Open.Enabled() || m.keys.Toggle.Enabled() {
		t.Error("the keys still belong to the local changes")
	}
}

func TestSwitchToLocalChangesFindsTheWorktreeOfThePullRequest(t *testing.T) {
	var model tea.Model = New(source.PullRequest{Target: "12"}, &fakeSender{}, nil, "dark", config.Config{})
	model, _ = model.Update(loadedMsg{gen: model.(Model).loadGen, review: source.Review{
		PullRequest: &github.PullRequest{HeadRefName: "feat/fix"},
	}})

	m, _ := press(model.(Model), "p")
	found := checkoutMsg{gen: m.loadGen, branch: "feat/fix", worktree: source.Worktree{Path: "/repo-fix", Branch: "feat/fix"}, found: true}

	// A search from an earlier switch must not move the checkout.
	stale := found
	stale.gen--
	model, _ = m.Update(stale)
	if dir := model.(Model).source.(source.Local).Dir; dir != "" {
		t.Errorf("a stale search moved the checkout to %q", dir)
	}

	model, _ = model.Update(found)
	if dir := model.(Model).source.(source.Local).Dir; dir != "/repo-fix" {
		t.Errorf("got dir %q, want the worktree of the pull request branch", dir)
	}

	missing := checkoutMsg{gen: m.loadGen, branch: "feat/fix"}
	model, _ = m.Update(missing)
	m = model.(Model)
	if m.source.(source.Local).Dir != "" || !strings.Contains(m.status, "feat/fix") {
		t.Errorf("got dir %q and status %q, want the current checkout and a notice", m.source.(source.Local).Dir, m.status)
	}
}

func TestHideTestsKeyHidesTestFilesAndSavesTheSetting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	review := testReview()
	review.Diff += `diff --git a/src/settings-mapper.spec.ts b/src/settings-mapper.spec.ts
--- a/src/settings-mapper.spec.ts
+++ b/src/settings-mapper.spec.ts
@@ -1,1 +1,1 @@
-it("maps")
+it("maps the language")
`
	var model tea.Model = New(fakeSource{review}, &fakeSender{}, nil, "dark", config.Config{})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 60})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	m, _ := press(model.(Model), "x")
	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "settings-mapper.spec.ts") || !strings.Contains(view, "1 test file hidden") {
		t.Errorf("the page still shows the test file:\n%s", view)
	}
	if !config.Load().HideTests {
		t.Error("the setting to hide test files was not saved")
	}

	m, _ = press(m, "x")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "settings-mapper.spec.ts") {
		t.Errorf("the page does not show the test file again:\n%s", view)
	}
}

func TestFindFileJumpsToTheChosenFile(t *testing.T) {
	review := testReview()
	review.Diff = ""
	for _, path := range []string{"cmd/main.go", "internal/ui/model.go", "internal/ui/model_test.go", "README.md"} {
		review.Diff += "diff --git a/" + path + " b/" + path + "\n--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n-old\n+new\n"
	}
	var model tea.Model = New(fakeSource{review}, &fakeSender{}, nil, "dark", config.Config{})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	model, _ = model.Update(loadedMsg{gen: 1, review: review})

	// Case does not matter, and the list keeps the page order.
	m, _ := press(model.(Model), "/", "U", "I", "/")
	if m.mode != finding {
		t.Fatal("finder did not open")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "internal/ui/model.go") || !strings.Contains(view, "internal/ui/model_test.go") || strings.Contains(view, "cmd/main.go") {
		t.Errorf("finder does not show only the matching files:\n%s", view)
	}

	m, _ = press(m, "down", "enter")
	if m.mode != browsing || m.cursor != m.fileStarts[2] {
		t.Errorf("got mode %d and cursor %d, want the title of the third file at %d", m.mode, m.cursor, m.fileStarts[2])
	}

	// Without a match, enter does nothing and esc goes back to the same place.
	m, _ = press(m, "/", "z", "z", "z", "enter")
	if m.mode != finding || !strings.Contains(ansi.Strip(m.View().Content), "No matching files.") {
		t.Errorf("finder closed or does not say that nothing matches:\n%s", ansi.Strip(m.View().Content))
	}
	m, _ = press(m, "esc")
	if m.mode != browsing || m.cursor != m.fileStarts[2] {
		t.Errorf("got mode %d and cursor %d after esc, want the page at %d", m.mode, m.cursor, m.fileStarts[2])
	}
}

func TestBaseFinderComparesWithTheChosenBranch(t *testing.T) {
	var model tea.Model = New(source.Local{Scope: source.Uncommitted}, &fakeSender{}, nil, "dark", config.Config{})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	model, _ = model.Update(branchesMsg{branches: []string{"feat/child", "feat/parent", "origin/feat/parent", "origin/main"}})

	m, _ := press(model.(Model), "p", "a", "r")
	if m.mode != choosingBase {
		t.Fatal("base finder did not open")
	}
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "origin/feat/parent") || strings.Contains(view, "origin/main") {
		t.Errorf("base finder does not show only the matching branches:\n%s", view)
	}

	// Choosing a base shows the branch changes, because only they compare with the base.
	m, cmd := press(m, "down", "enter")
	local := m.source.(source.Local)
	if m.mode != browsing || local.Base != "origin/feat/parent" || local.Scope != source.Branch || cmd == nil {
		t.Errorf("got mode %d, base %q, and scope %v, want the branch changes compared to origin/feat/parent to load", m.mode, local.Base, local.Scope)
	}
}

func TestEditorLineOpensARemovedLineAtTheNextLine(t *testing.T) {
	file := diff.Parse(testDiff)[0]

	// Line 2 of the hunk is the removed META_TEXT_MAX_LENGTH, and matchLanguage takes its place at line 3.
	if got := editorLine(file, row{hunk: 0, line: 2}); got != 3 {
		t.Errorf("got line %d, want 3", got)
	}
}

func TestEditorKeyIsOffUntilAnEditorIsFound(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 40)
	m.cursor = m.fileStarts[0]

	m, _ = press(m, "e")
	if m.status != "" || m.keys.Edit.Enabled() {
		t.Errorf("got status %q; want the editor key off", m.status)
	}
}

func TestEditorKeyNeedsTheBranchOfThePullRequest(t *testing.T) {
	m := newLoadedModel(t, &fakeSender{}, 60, 40)
	m.cursor = m.fileStarts[0]

	var model tea.Model = m
	model, _ = model.Update(editor.Editor{})
	m = model.(Model)
	if !m.keys.Edit.Enabled() {
		t.Error("the editor key is off after an editor was found")
	}

	m, _ = press(m, "e")
	if !strings.Contains(m.status, "Check out the branch") {
		t.Errorf("got status %q, want a hint to check out the branch", m.status)
	}
}
