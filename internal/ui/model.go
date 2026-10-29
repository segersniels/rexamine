// Package ui is the review screen of rexamine: a scrollable page with the
// review's details and its diff, a line cursor, line selection, and a
// composer that sends comments on the selected lines to an agent.
package ui

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexamine/internal/agent"
	"github.com/segersniels/rexamine/internal/config"
	"github.com/segersniels/rexamine/internal/diff"
	"github.com/segersniels/rexamine/internal/editor"
	"github.com/segersniels/rexamine/internal/github"
	"github.com/segersniels/rexamine/internal/source"
	"github.com/segersniels/rexamine/internal/update"
)

// watchInterval is how often a local review checks the repository for changes.
const watchInterval = 2 * time.Second

// statusDuration is how long a status message stays in the footer.
const statusDuration = 3 * time.Second

// focusCooldown is the shortest time between two loads that a pull request
// starts when its pane gets focus.
const focusCooldown = 30 * time.Second

// editorStartup is how long rexamine waits for an editor launcher to fail
// before it takes the editor as open.
const editorStartup = time.Second

// composerIndent is how far the comment box is indented under the code.
const composerIndent = 2

// mode tells whether keys go to the page, to the comment composer, to the
// worktree picker, to the file finder, or to the base branch finder.
type mode int

const (
	browsing mode = iota
	composing
	picking
	finding
	choosingBase
)

// destination tells where the composer sends a comment.
type destination int

const (
	toAgent destination = iota
	toPullRequest
)

type (
	loadedMsg struct {
		gen    int
		review source.Review
		err    error
	}
	watchMsg    struct{ gen int }
	turnMsg     struct{ event agent.TurnEvent }
	snapshotMsg struct {
		tree string
		err  error
	}
	sentMsg struct {
		err  error
		done string
	}
	openedMsg    struct{ err error }
	worktreesMsg struct {
		worktrees []source.Worktree
		err       error
	}
	branchesMsg struct {
		branches []string
		err      error
	}
	checkoutMsg struct {
		gen      int
		branch   string
		worktree source.Worktree
		found    bool
		err      error
	}
	clearStatusMsg struct{ id int }
)

// lineKey identifies a diff line across layouts and reloads, so the cursor
// and selection stay on the same line when the page changes.
type lineKey struct {
	path             string
	kind             diff.Kind
	oldLine, newLine int
}

// Model is the Bubble Tea model of the review screen.
type Model struct {
	source source.Source

	// other is the view that the switch key shows: the pull request of the branch for local changes,
	// or the local changes for a pull request. It is nil when there is nothing to switch to.
	other         source.Source
	sender        agent.Sender
	markdownStyle string

	// turns reports when the agent starts or ends a turn, or is nil without an agent to watch.
	turns <-chan agent.TurnEvent

	keys        browseKeys
	composeKeys composeKeys
	pickKeys    pickKeys
	findKeys    findKeys
	baseKeys    findKeys
	help        help.Model
	spinner     spinner.Model
	composer    textarea.Model

	review *source.Review

	// parsed holds every file of the diff, and files holds the files that the page shows.
	parsed  []diff.File
	files   []diff.File
	loadErr error

	// settings last between runs, such as whether the page hides test files.
	settings config.Config

	// highlights holds the syntax colors of files. They are computed once per
	// load, because a new layout on each resize must stay fast.
	highlights []highlightedFile

	// highlightCache holds the colors of the shown hunks, so a reload highlights only the hunks that changed.
	highlightCache hunkColors

	// wholeFiles holds the colors of the full files, by path, which replace the hunk colors when they arrive.
	// wholeFilesGen counts the background jobs that color them, so the result of an older job is dropped.
	wholeFiles    map[string]wholeFileColors
	wholeFilesGen int

	loading  bool
	loadGen  int
	watchGen int

	// loadStarted is when the last load started, so a focus reload can wait for the cooldown.
	loadStarted time.Time

	width, height int
	rows          []row
	fileStarts    []int

	// collapsed holds the paths of the collapsed files. Paths stay the same
	// across reloads, so a file stays collapsed when the diff changes.
	collapsed map[string]bool

	// descriptionExpanded shows the full pull request description. It starts
	// collapsed, so the diff is the first thing on the page.
	descriptionExpanded bool

	offset int
	cursor int

	// anchor is the row where the selection starts, or -1 without a selection.
	anchor   int
	dragging bool

	// pressedGutter is set when a mouse press started on a line number.
	pressedGutter bool
	mode          mode
	snippet       agent.Snippet

	// linesChanged is set when a reload changed the lines of the open comment, so the comment points at old code.
	linesChanged bool

	// destination stays the same between comments, so a review pass can post several comments in a row.
	destination destination

	// worktrees holds the checkouts that the picker shows, and pick is the one under its cursor.
	worktrees []source.Worktree
	pick      int

	// branches holds the branches that the base branch finder searches.
	branches []string

	// finder holds the query of the file or base branch finder, and selectedMatch is the match under its cursor.
	finder        textinput.Model
	selectedMatch int

	status   string
	statusID int

	// newRelease tells about a newer release of rexamine, or is nil.
	newRelease *update.Notice

	// editor opens files at a line, or is nil until main finds the user's editor.
	editor *editor.Editor

	theme terminalTheme
}

// New creates the review screen. Comments go to the sender. turns reports when
// the agent starts a turn, or is nil without an agent to watch. markdownStyle
// is a glamour style name, such as "dark". settings are the saved settings.
func New(src source.Source, sender agent.Sender, turns <-chan agent.TurnEvent, markdownStyle string, settings config.Config) Model {
	// The components start with dark styles, so they get the styles for the terminal background.
	isDark := markdownStyle != "light"

	composer := textarea.New()
	composer.ShowLineNumbers = false
	composer.Prompt = ""
	composer.CharLimit = 0
	composer.SetHeight(3)
	composer.KeyMap.InsertNewline.SetKeys("alt+enter", "ctrl+j")
	composerStyles := textarea.DefaultStyles(isDark)
	composerStyles.Focused.CursorLine = lipgloss.NewStyle()
	composer.SetStyles(composerStyles)

	finder := textinput.New()
	finder.Prompt = "/ "
	finder.SetStyles(textinput.DefaultStyles(isDark))

	helpView := help.New()
	helpView.Styles = help.DefaultStyles(isDark)

	keys := newBrowseKeys()
	keys.Edit.SetEnabled(false)
	baseKeys := newFindKeys()
	baseKeys.Jump.SetHelp("enter", "compare")

	// The pull request of the current branch is what gh shows without a target.
	var other source.Source
	switch src.(type) {
	case source.Local:
		other = source.PullRequest{}
	case source.PullRequest:
		other = source.Local{}
	}

	m := Model{
		source:        src,
		other:         other,
		sender:        sender,
		markdownStyle: markdownStyle,
		settings:      settings,
		turns:         turns,
		keys:          keys,
		composeKeys:   newComposeKeys(),
		pickKeys:      newPickKeys(),
		findKeys:      newFindKeys(),
		baseKeys:      baseKeys,
		help:          helpView,
		spinner:       spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(accentStyle)),
		composer:      composer,
		finder:        finder,
		collapsed:     map[string]bool{},
		loading:       true,
		loadGen:       1,
		loadStarted:   time.Now(),
		anchor:        -1,
		theme:         newTerminalTheme(),
	}
	m.enableKeys()
	return m
}

// enableKeys turns on the keys that work for the current source, and picks
// where comments go.
func (m *Model) enableKeys() {
	_, local := m.source.(source.Local)
	m.keys.Toggle.SetEnabled(local)
	m.keys.Worktree.SetEnabled(local)
	m.keys.Base.SetEnabled(local)
	m.keys.Open.SetEnabled(!local)
	m.keys.Switch.SetEnabled(m.other != nil)
	if local {
		m.keys.Switch.SetHelp("p", "pull request")
	} else {
		m.keys.Switch.SetHelp("p", "local changes")
	}

	// A pull request opened without an agent has nobody to ask, so its comments only go to GitHub.
	_, toClipboard := m.sender.(agent.Clipboard)
	m.composeKeys.Cycle.SetEnabled(!local && !toClipboard)
	m.setDestination(toAgent)
	if !local && toClipboard {
		m.setDestination(toPullRequest)
	}
}

// Init starts the first load.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadReview(m.source, m.loadGen), m.spinner.Tick, requestTerminalTheme()}
	if m.turns != nil {
		cmds = append(cmds, waitForTurn(m.turns))
	}
	return tea.Batch(cmds...)
}

// Update handles a message and returns the next state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// A hidden pane can get a size of zero. It has nothing to draw, so the page keeps its last size and
		// a reload while it is hidden still lays out rows for the new files.
		if msg.Width == 0 || msg.Height == 0 {
			return m, nil
		}
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(msg.Width)
		m.composer.SetWidth(m.composerTextWidth())
		m.layout(m.files)
		return m, nil

	case loadedMsg:
		return m.handleLoaded(msg)

	case wholeFilesMsg:
		if msg.gen != m.wholeFilesGen {
			return m, nil
		}
		m.wholeFiles = msg.colors
		m.showFiles()
		return m, nil

	case tea.FocusMsg:
		// Local reviews already check for changes on a timer. A pull request reloads only on request or focus.
		_, local := m.source.(source.Local)
		if local || time.Since(m.loadStarted) < focusCooldown {
			return m, nil
		}
		return m, m.reload()

	case watchMsg:
		if msg.gen != m.watchGen {
			return m, nil
		}
		return m, m.reload()

	case turnMsg:
		local := m.localSource()
		if msg.event == agent.TurnEnded {
			local.TurnEndedAt = time.Now()
			return m.updateLocalSource(local, waitForTurn(m.turns))
		}

		local.TurnStartedAt, local.TurnEndedAt = time.Now(), time.Time{}
		local.JoinedTurn = msg.event == agent.TurnJoined
		return m.updateLocalSource(local, takeSnapshot(local.Dir), waitForTurn(m.turns))

	case snapshotMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render("Save the start of the agent's turn: " + msg.err.Error()))
		}
		local := m.localSource()

		// The turn that just ended changed files, so its start becomes the fallback for a turn without edits.
		if local.TurnStart != "" && msg.tree != local.TurnStart {
			local.PreviousTurnStart = local.TurnStart
		}
		local.TurnStart = msg.tree
		return m.updateLocalSource(local)

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sentMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render(msg.err.Error()))
		}
		return m, m.setStatus(statusStyle.Render("✓ " + msg.done))

	case openedMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render(msg.err.Error()))
		}
		return m, nil

	// main sends the editor when its search at startup finds one, and only then does the editor key work.
	case editor.Editor:
		m.editor = &msg
		m.keys.Edit.SetEnabled(true)
		return m, nil

	case worktreesMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render(msg.err.Error()))
		}

		// The current checkout is always in the list, so one entry leaves nothing to switch to.
		if len(msg.worktrees) < 2 {
			return m, m.setStatus(dimStyle.Render("This repository has no other worktrees."))
		}
		m.worktrees, m.pick, m.mode = msg.worktrees, 0, picking
		return m, nil

	case branchesMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render(msg.err.Error()))
		}
		m.branches = msg.branches
		return m, m.startFinder(choosingBase)

	case checkoutMsg:
		// A newer switch or load replaced this search.
		if msg.gen != m.loadGen {
			return m, nil
		}

		if msg.found && !msg.worktree.Current {
			return m.switchWorktree(msg.worktree)
		}

		// Without a checkout of the branch, the current checkout is the best match. A failed search
		// shows the current checkout too, and its load reports any problem with the repository.
		if msg.err == nil && !msg.found {
			return m, tea.Batch(m.reload(), m.setStatus(dimStyle.Render("No worktree has "+msg.branch+" checked out.")))
		}
		return m, m.reload()

	// main sends the notice when its check finds a newer release.
	case update.Notice:
		m.newRelease = &msg
		m.clamp()
		return m, nil

	case clearStatusMsg:
		if msg.id == m.statusID {
			m.status = ""
			m.clamp()
		}
		return m, nil

	case tea.BackgroundColorMsg, uv.UnknownOscEvent:
		m.theme = m.theme.withReply(msg)
		return m, nil

	case tea.PasteMsg:
		// Pasted text goes to the box that takes text. The page ignores it.
		var cmd tea.Cmd
		switch m.mode {
		case composing:
			m.composer, cmd = m.composer.Update(msg)
		case finding, choosingBase:
			m.finder, cmd = m.finder.Update(msg)
			m.selectedMatch = 0
		}
		return m, cmd

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case composing:
			return m.updateComposer(msg)
		case picking:
			return m.updatePicker(msg)
		case finding, choosingBase:
			return m.updateFinder(msg)
		}
		return m.updateBrowser(msg)

	case tea.MouseMsg:
		if m.mode == picking || m.mode == finding || m.mode == choosingBase {
			return m, nil
		}
		return m, m.handleMouse(msg)
	}

	return m, nil
}

// handleLoaded shows a loaded review and schedules the next local check.
func (m Model) handleLoaded(msg loadedMsg) (tea.Model, tea.Cmd) {
	// A newer load replaced this one, for example after a scope toggle.
	if msg.gen != m.loadGen {
		return m, nil
	}

	m.loading = false
	watch := m.scheduleWatch()

	if msg.err != nil {
		if m.review == nil {
			m.loadErr = msg.err
			return m, watch
		}
		return m, tea.Batch(watch, m.setStatus(errorStyle.Render(msg.err.Error())))
	}

	// Watch checks find no change most of the time, so keep the page as it is.
	unchanged := m.review != nil && msg.review.PullRequest == nil && msg.review.Title == m.review.Title &&
		msg.review.Base == m.review.Base && msg.review.Head == m.review.Head && msg.review.Diff == m.review.Diff
	if unchanged {
		return m, watch
	}

	m.loadErr = nil
	m.review = &msg.review
	m.parsed = diff.Parse(msg.review.Diff)
	m.showFiles()

	// An edit can move or change the commented code while the user writes, and the draft keeps the old lines.
	if m.mode == composing {
		m.linesChanged = m.anchor < 0 || !slices.Equal(m.selectedSnippet().Lines, m.snippet.Lines)
	}

	m.wholeFilesGen++
	if msg.review.Files == nil {
		return m, watch
	}
	return m, tea.Batch(watch, colorWholeFiles(m.wholeFilesGen, msg.review.Files, m.parsed, m.markdownStyle))
}

// showFiles lays out the parsed files. It leaves out the test files when the
// settings hide them.
func (m *Model) showFiles() {
	files := m.parsed
	if m.settings.HideTests {
		files = slices.DeleteFunc(slices.Clone(files), diff.File.IsTest)
	}
	m.highlights, m.highlightCache = highlightFiles(files, m.markdownStyle, m.highlightCache, m.wholeFiles)
	m.layout(files)
}

// updateBrowser handles keys while the user reads the page.
func (m Model) updateBrowser(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	page := m.bodyHeight()

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.HalfDown):
		m.moveCursor(page / 2)
	case key.Matches(msg, m.keys.HalfUp):
		m.moveCursor(-page / 2)
	case key.Matches(msg, m.keys.PageDown):
		m.moveCursor(page)
	case key.Matches(msg, m.keys.PageUp):
		m.moveCursor(-page)
	case key.Matches(msg, m.keys.Top):
		m.moveCursor(-len(m.rows))
	case key.Matches(msg, m.keys.Bottom):
		m.moveCursor(len(m.rows))
	case key.Matches(msg, m.keys.NextFile):
		m.jumpToFile(1)
	case key.Matches(msg, m.keys.PrevFile):
		m.jumpToFile(-1)
	case key.Matches(msg, m.keys.FindFile):
		return m, m.startFinder(finding)
	case key.Matches(msg, m.keys.Collapse):
		if m.cursor < len(m.rows) && m.rows[m.cursor].kind == descriptionRow {
			m.toggleDescription()
		} else {
			m.toggleFile(m.cursor)
		}
	case key.Matches(msg, m.keys.HideTests):
		m.settings.HideTests = !m.settings.HideTests
		m.showFiles()
		if err := config.Save(m.settings); err != nil {
			return m, m.setStatus(errorStyle.Render("Save the settings: " + err.Error()))
		}
		if m.settings.HideTests {
			return m, m.setStatus(dimStyle.Render("Hiding test files."))
		}
		return m, m.setStatus(dimStyle.Render("Showing test files."))
	case key.Matches(msg, m.keys.Select):
		return m, m.toggleSelection()
	case key.Matches(msg, m.keys.Clear):
		m.anchor = -1
	case key.Matches(msg, m.keys.Comment):
		return m, m.startComment()
	case key.Matches(msg, m.keys.Edit):
		return m, m.openEditor()
	case key.Matches(msg, m.keys.Toggle):
		local := m.source.(source.Local)
		local.Scope = local.Scope.Next()

		// Without an agent to watch, there are no turns to show.
		if local.Scope == source.LastTurn && m.turns == nil {
			local.Scope = local.Scope.Next()
		}
		m.source = local
		return m, tea.Batch(m.reload(), m.setStatus(dimStyle.Render("Showing "+local.Scope.String()+"…")))
	case key.Matches(msg, m.keys.Worktree):
		return m, listWorktrees(m.source.(source.Local).Dir)
	case key.Matches(msg, m.keys.Base):
		return m, listBranches(m.source.(source.Local).Dir)
	case key.Matches(msg, m.keys.Switch):
		return m.switchSource()
	case key.Matches(msg, m.keys.Reload):
		return m, tea.Batch(m.reload(), m.setStatus(dimStyle.Render("Reloading…")))
	case key.Matches(msg, m.keys.Open):
		if m.review != nil && m.review.PullRequest != nil {
			return m, openInBrowser(m.review.PullRequest.URL)
		}
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		m.clamp()
	}

	return m, nil
}

// updatePicker handles keys while the user chooses a worktree.
func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.pickKeys.Up):
		m.pick = max(0, m.pick-1)
	case key.Matches(msg, m.pickKeys.Down):
		m.pick = min(len(m.worktrees)-1, m.pick+1)
	case key.Matches(msg, m.pickKeys.Cancel):
		m.mode = browsing
		m.clamp()
	case key.Matches(msg, m.pickKeys.Switch):
		return m.switchWorktree(m.worktrees[m.pick])
	}
	return m, nil
}

// startFinder opens the file finder or the base branch finder, as mode
// tells, with an empty query.
func (m *Model) startFinder(mode mode) tea.Cmd {
	m.finder.Placeholder = "Type part of a path…"
	if mode == choosingBase {
		m.finder.Placeholder = "Type part of a branch…"
	}
	if len(m.finderItems(mode)) == 0 {
		return nil
	}

	m.mode = mode
	m.selectedMatch = 0
	m.finder.Reset()
	return m.finder.Focus()
}

// updateFinder handles keys while the user finds a file or a base branch.
// Other keys edit the query, and a new query moves the cursor back to the
// first match.
func (m Model) updateFinder(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	matches := m.finderMatches()

	switch {
	case key.Matches(msg, m.findKeys.Up):
		m.selectedMatch = max(0, m.selectedMatch-1)
		return m, nil
	case key.Matches(msg, m.findKeys.Down):
		m.selectedMatch = clamp(m.selectedMatch+1, 0, len(matches)-1)
		return m, nil
	case key.Matches(msg, m.findKeys.Cancel):
		m.mode = browsing
		m.finder.Blur()
		return m, nil
	case key.Matches(msg, m.findKeys.Jump):
		if len(matches) == 0 {
			return m, nil
		}
		chosen := matches[min(m.selectedMatch, len(matches)-1)]
		if m.mode == choosingBase {
			return m.compareWithBase(m.branches[chosen])
		}
		m.mode = browsing
		m.finder.Blur()

		// A reload can remove files while the finder is open, so the cursor can be past the last match.
		title := m.fileStarts[chosen]
		m.cursor, m.offset = title, title
		m.clamp()
		return m, nil
	}

	query := m.finder.Value()
	var cmd tea.Cmd
	m.finder, cmd = m.finder.Update(msg)
	if m.finder.Value() != query {
		m.selectedMatch = 0
	}
	return m, cmd
}

// finderItems lists what the finder in mode searches: the paths of the shown
// files in page order, or the branches.
func (m Model) finderItems(mode mode) []string {
	if mode == choosingBase {
		return m.branches
	}
	paths := make([]string, len(m.files))
	for i, file := range m.files {
		paths[i] = file.Path
	}
	return paths
}

// finderMatches lists the indexes of the finder items that contain the
// query. Case does not matter.
func (m Model) finderMatches() []int {
	query := strings.ToLower(m.finder.Value())
	var matches []int
	for i, item := range m.finderItems(m.mode) {
		if strings.Contains(strings.ToLower(item), query) {
			matches = append(matches, i)
		}
	}
	return matches
}

// compareWithBase shows the branch changes compared to the chosen branch,
// such as the parent of a stacked branch.
func (m Model) compareWithBase(branch string) (tea.Model, tea.Cmd) {
	m.mode = browsing
	m.finder.Blur()

	local := m.source.(source.Local)
	local.Base, local.Scope = branch, source.Branch
	m.source = local
	return m, tea.Batch(m.reload(), m.setStatus(dimStyle.Render("Comparing with "+branch+"…")))
}

// switchWorktree shows the local changes of another checkout. The turn
// snapshots, collapsed files, and cursor belong to the old checkout, so they
// start over.
func (m Model) switchWorktree(w source.Worktree) (tea.Model, tea.Cmd) {
	m.mode = browsing
	if w.Current {
		m.clamp()
		return m, nil
	}

	local := m.source.(source.Local)
	local.Dir = w.Path
	local.TurnStart, local.PreviousTurnStart = "", ""
	m.source = local

	// The switch key shows the pull request of the branch in the new checkout.
	if _, ok := m.other.(source.PullRequest); ok {
		m.other = source.PullRequest{Dir: w.Path}
	}

	m.review, m.loadErr, m.files, m.rows, m.fileStarts = nil, nil, nil, nil, nil
	m.collapsed = map[string]bool{}
	m.cursor, m.offset, m.anchor = 0, 0, -1
	m.loading = true
	return m, tea.Batch(m.reload(), m.spinner.Tick)
}

// switchSource swaps between the local changes and the pull request. Each
// side keeps its own settings, such as the local scope and the turn
// snapshots. The page and the cursor belong to the old diff, so they start over.
func (m Model) switchSource() (tea.Model, tea.Cmd) {
	var branch string
	if m.review != nil && m.review.PullRequest != nil {
		branch = m.review.PullRequest.HeadRefName
	}

	m.source, m.other = m.other, m.source
	m.enableKeys()

	// A local check that is still planned would reload the pull request.
	m.watchGen++

	m.review, m.loadErr, m.files, m.rows, m.fileStarts = nil, nil, nil, nil, nil
	m.collapsed = map[string]bool{}
	m.descriptionExpanded = false
	m.cursor, m.offset, m.anchor = 0, 0, -1
	m.loading = true

	// The local changes of a pull request are in the checkout of its branch, which can be another worktree.
	local, toLocal := m.source.(source.Local)
	if toLocal && branch != "" {
		m.loadGen++
		return m, tea.Batch(findCheckout(local.Dir, branch, m.loadGen), m.spinner.Tick)
	}
	return m, tea.Batch(m.reload(), m.spinner.Tick)
}

// updateLocalSource stores a change to the turn of the local changes, and
// shows it when the last turn shows now. It runs cmds beside the reload.
func (m Model) updateLocalSource(local source.Local, cmds ...tea.Cmd) (tea.Model, tea.Cmd) {
	// The pull request shows now, so the change waits for a switch back to the local changes.
	if _, showing := m.source.(source.Local); !showing {
		m.other = local
		return m, tea.Batch(cmds...)
	}

	m.source = local
	if local.Scope != source.LastTurn {
		return m, tea.Batch(cmds...)
	}
	return m, tea.Batch(append(cmds, m.reload())...)
}

// localSource returns the local changes, whether they show now or wait
// behind the switch key.
func (m Model) localSource() source.Local {
	if local, ok := m.source.(source.Local); ok {
		return local
	}
	return m.other.(source.Local)
}

// updateComposer handles keys while the user writes a comment.
func (m Model) updateComposer(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.composeKeys.Cycle):
		if m.destination == toAgent {
			m.setDestination(toPullRequest)
		} else {
			m.setDestination(toAgent)
		}
		return m, nil

	// The comment points at code that changed, so the user selects the lines again.
	case m.linesChanged && (key.Matches(msg, m.composeKeys.Send) || key.Matches(msg, m.composeKeys.SendNow)):
		return m, m.setStatus(errorStyle.Render("The code changed. Select the lines again to comment."))

	case key.Matches(msg, m.composeKeys.SendNow):
		text := agent.Format(m.snippet, m.composer.Value())
		m.endComment()
		return m, submit(m.sender.(agent.Submitter), text)

	case key.Matches(msg, m.composeKeys.Send) && m.destination == toPullRequest:
		body := strings.TrimSpace(m.composer.Value())

		// GitHub rejects a comment without text, and the lines alone say nothing to other reviewers.
		if body == "" {
			return m, m.setStatus(dimStyle.Render("Write a comment to post it on GitHub."))
		}
		comment := reviewComment(m.snippet, body)
		m.endComment()
		return m, postComment(*m.review.PullRequest, comment)

	case key.Matches(msg, m.composeKeys.Send):
		text := agent.Format(m.snippet, m.composer.Value())
		m.endComment()
		return m, send(m.sender, text)

	case key.Matches(msg, m.composeKeys.Cancel):
		m.endComment()
		return m, nil
	}

	var cmd tea.Cmd
	m.composer, cmd = m.composer.Update(msg)
	return m, cmd
}

// handleMouse scrolls with the wheel, moves the cursor with a click, and
// selects lines with a drag. Releasing a drag, or clicking a line number,
// opens the comment box for the selected lines.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	mouse := msg.Mouse()
	_, press := msg.(tea.MouseClickMsg)
	_, motion := msg.(tea.MouseMotionMsg)
	_, release := msg.(tea.MouseReleaseMsg)

	switch {
	case mouse.Button == tea.MouseWheelUp:
		m.scrollBy(-3)
	case mouse.Button == tea.MouseWheelDown:
		m.scrollBy(3)

	case mouse.Button == tea.MouseLeft && press:
		i, ok := m.rowAt(mouse.Y)
		if !ok {
			return nil
		}

		// A click on the page closes an open comment box without sending it.
		if m.mode == composing {
			m.endComment()
		}

		// A click on the description heading or its "more lines" row expands or collapses the description.
		if m.rows[i].kind == descriptionRow {
			m.toggleDescription()
			return nil
		}

		// A click on a file title collapses or expands the file.
		if r := m.rows[i]; r.kind == fileRow && r.hunk == -1 {
			m.toggleFile(i)
			return nil
		}

		m.cursor = i
		m.anchor = -1
		m.dragging = m.rows[i].kind == codeRow
		m.pressedGutter = m.dragging && mouse.X >= 1 && mouse.X <= ansi.StringWidth(m.rows[i].gutter)
		if m.dragging {
			m.anchor = i
		}

	case motion && m.dragging:
		m.cursor = m.dragTarget(mouse.Y)
		m.clamp()

	case release && m.dragging:
		m.dragging = false

		// A click on the code, not on a line number, only moves the cursor.
		if m.anchor == m.cursor && !m.pressedGutter {
			m.anchor = -1
			return nil
		}
		return m.startComment()
	}

	return nil
}

// dragTarget finds the row a drag has reached at screen line y. Above or
// below the page it returns the row past the edge, so the page scrolls.
func (m Model) dragTarget(y int) int {
	if i, ok := m.rowAt(y); ok {
		return i
	}
	if y < 1 {
		return m.offset - 1
	}
	return m.offset + m.bodyHeight()
}

// reload loads the review again. The result replaces any earlier load.
func (m *Model) reload() tea.Cmd {
	m.loadGen++
	m.loadStarted = time.Now()
	return loadReview(m.source, m.loadGen)
}

// scheduleWatch plans the next check for local changes. Pull requests change
// rarely, so they reload only on request or when the pane gets focus.
func (m *Model) scheduleWatch() tea.Cmd {
	if _, local := m.source.(source.Local); !local {
		return nil
	}

	m.watchGen++
	gen := m.watchGen
	return tea.Tick(watchInterval, func(time.Time) tea.Msg { return watchMsg{gen} })
}

// layout lays out the page for files at the current width, and keeps the
// cursor and selection on the same diff lines.
func (m *Model) layout(files []diff.File) {
	if m.review == nil || m.width == 0 {
		m.files = files
		return
	}

	cursorKey, cursorOnLine := m.lineKeyAt(m.cursor)
	anchorKey, anchorOnLine := m.lineKeyAt(m.anchor)
	screenRow := m.cursor - m.offset
	progress := 0.0
	if len(m.rows) > 0 {
		progress = float64(m.cursor) / float64(len(m.rows))
	}

	m.files = files
	hiddenTests := len(m.parsed) - len(files)
	m.rows, m.fileStarts = layoutPage(*m.review, files, hiddenTests, m.highlights, m.contentWidth(), m.markdownStyle, m.collapsed, m.descriptionExpanded)

	m.cursor = int(progress * float64(len(m.rows)))
	if i, ok := m.findLine(cursorKey); cursorOnLine && ok {
		m.cursor = i
	}

	m.anchor = -1
	if i, ok := m.findLine(anchorKey); anchorOnLine && ok {
		m.anchor = i
	}

	m.offset = m.cursor - screenRow
	m.clamp()
}

// lineKeyAt returns the key of the diff line on row i, if row i is a code row.
func (m *Model) lineKeyAt(i int) (lineKey, bool) {
	if i < 0 || i >= len(m.rows) || m.rows[i].kind != codeRow {
		return lineKey{}, false
	}

	r := m.rows[i]
	line := m.files[r.file].Hunks[r.hunk].Lines[r.line]
	return lineKey{m.files[r.file].Path, line.Kind, line.OldLine, line.NewLine}, true
}

// findLine finds the first row that shows the diff line with the key. It
// searches only the rows of the key's file, because a large diff has many rows.
func (m *Model) findLine(target lineKey) (int, bool) {
	for f, start := range m.fileStarts {
		if m.files[f].Path != target.path {
			continue
		}

		end := len(m.rows)
		if f+1 < len(m.fileStarts) {
			end = m.fileStarts[f+1]
		}
		for i := start; i < end; i++ {
			if key, ok := m.lineKeyAt(i); ok && key == target {
				return i, true
			}
		}
	}
	return 0, false
}

// moveCursor moves the cursor by n rows and scrolls to keep it visible.
func (m *Model) moveCursor(n int) {
	m.cursor += n
	m.clamp()
}

// scrollBy scrolls the page by n rows and keeps the cursor on the screen.
// While the comment box is open, the cursor and selection stay in place.
func (m *Model) scrollBy(n int) {
	height := m.bodyHeight()
	m.offset = clamp(m.offset+n, 0, max(0, len(m.rows)+m.composerHeight()-height))
	if m.mode == composing {
		return
	}
	m.cursor = clamp(m.cursor, m.offset, m.offset+height-1)
	m.cursor = clamp(m.cursor, 0, max(0, len(m.rows)-1))
}

// clamp keeps the cursor on the page and scrolls the page to the cursor, or
// to the selection and its comment box while the box is open.
func (m *Model) clamp() {
	if len(m.rows) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}

	height := m.bodyHeight()
	m.cursor = clamp(m.cursor, 0, len(m.rows)-1)

	// The bottom counts screen lines, so it includes the comment box below the selection.
	top, bottom := m.cursor, m.cursor
	if first, _, ok := m.selection(); ok && m.mode == composing {
		top, bottom = first, m.selectionEnd()+m.composerHeight()
	}

	m.offset = clamp(m.offset, bottom-height+1, top)
	m.offset = clamp(m.offset, 0, max(0, len(m.rows)+m.composerHeight()-height))
}

// jumpToFile moves the cursor to the next or previous file's title and
// scrolls it to the top of the screen.
func (m *Model) jumpToFile(direction int) {
	target := -1
	for _, start := range m.fileStarts {
		if direction > 0 && start > m.cursor {
			target = start
			break
		}
		if direction < 0 && start < m.cursor {
			target = start
		}
	}
	if target < 0 {
		return
	}

	m.cursor, m.offset = target, target
	m.clamp()
}

// toggleFile collapses or expands the file on row i, and moves the cursor to
// the file's title so it stays on a visible row.
func (m *Model) toggleFile(i int) {
	if i >= len(m.rows) || m.rows[i].file < 0 {
		return
	}

	file := m.rows[i].file
	path := m.files[file].Path
	m.collapsed[path] = !m.collapsed[path]
	m.anchor = -1

	// The rows above the file do not change, so the title keeps its row and the screen does not jump.
	offset := m.offset
	m.layout(m.files)
	m.cursor, m.offset = m.fileStarts[file], offset
	m.clamp()
}

// toggleDescription expands or collapses the pull request description, and
// moves the cursor to the description heading so it stays on a visible row.
func (m *Model) toggleDescription() {
	m.descriptionExpanded = !m.descriptionExpanded
	m.anchor = -1

	// The rows above the heading do not change, so the heading keeps its row and the screen does not jump.
	offset := m.offset
	m.layout(m.files)
	for i, r := range m.rows {
		if r.kind == descriptionRow {
			m.cursor = i
			break
		}
	}
	m.offset = offset
	m.clamp()
}

// toggleSelection starts a selection at the cursor, or clears it.
func (m *Model) toggleSelection() tea.Cmd {
	if m.anchor >= 0 {
		m.anchor = -1
		return nil
	}
	if m.cursor >= len(m.rows) || m.rows[m.cursor].kind != codeRow {
		return m.setStatus(dimStyle.Render("Move to a diff line to start a selection."))
	}

	m.anchor = m.cursor
	return nil
}

// selection returns the first and last selected row.
func (m Model) selection() (first, last int, ok bool) {
	if m.anchor < 0 || m.anchor >= len(m.rows) {
		return 0, 0, false
	}
	return min(m.anchor, m.cursor), max(m.anchor, m.cursor), true
}

// isSelected reports whether row i is a selected diff line. A selection only
// covers the file where it started.
func (m Model) isSelected(i int) bool {
	first, last, ok := m.selection()
	return ok && i >= first && i <= last &&
		m.rows[i].kind == codeRow && m.rows[i].file == m.rows[m.anchor].file
}

// selectedSnippet collects the selected diff lines.
func (m Model) selectedSnippet() agent.Snippet {
	first, last, _ := m.selection()
	file := m.files[m.rows[m.anchor].file]

	var lines []diff.Line
	var previous row
	for i := first; i <= last; i++ {
		r := m.rows[i]

		// A wrapped line spans several rows, and the snippet needs it once.
		if !m.isSelected(i) || r.sameLine(previous) {
			continue
		}
		previous = r
		lines = append(lines, file.Hunks[r.hunk].Lines[r.line])
	}

	return agent.Snippet{Path: file.Path, Lines: lines}
}

// startComment opens the composer for the selection, or for the line under
// the cursor when nothing is selected.
func (m *Model) startComment() tea.Cmd {
	if m.anchor < 0 {
		if m.cursor >= len(m.rows) || m.rows[m.cursor].kind != codeRow {
			return m.setStatus(dimStyle.Render("Move to a diff line, or select lines with v, to comment."))
		}
		m.anchor = m.cursor
	}

	m.snippet = m.selectedSnippet()
	m.linesChanged = false
	m.mode = composing
	m.composer.Reset()
	m.composer.SetWidth(m.composerTextWidth())
	m.clamp()
	return m.composer.Focus()
}

// openEditor opens the file under the cursor in the user's editor, at the
// cursor's line or at the first selected line.
func (m *Model) openEditor() tea.Cmd {
	i := m.cursor
	if first, last, ok := m.selection(); ok {
		// The selection covers only the file where it started, which can be below the first selected row.
		i = first
		for i < last && !m.isSelected(i) {
			i++
		}
	}
	if i >= len(m.rows) || m.rows[i].file < 0 {
		return m.setStatus(dimStyle.Render("Move to a file to open it in your editor."))
	}

	// The files of a pull request are on disk only where its branch is checked out.
	if m.review.Root == "" {
		return m.setStatus(dimStyle.Render("Check out the branch of the pull request to open its files."))
	}

	r := m.rows[i]
	file := m.files[r.file]
	if file.IsDeleted {
		return m.setStatus(dimStyle.Render("This file was deleted, so there is nothing to open."))
	}

	return startEditor(*m.editor, m.review.Root, file.Path, editorLine(file, r))
}

// editorLine finds the line of the new file that row r shows. A removed line
// is not in the new file, so it opens at the next line that is, or at the
// line before it at the end of a hunk. A file title opens at the start.
func editorLine(file diff.File, r row) int {
	if r.hunk < 0 {
		return 1
	}

	lines := file.Hunks[r.hunk].Lines
	for _, line := range lines[r.line:] {
		if line.NewLine > 0 {
			return line.NewLine
		}
	}
	for i := r.line - 1; i >= 0; i-- {
		if lines[i].NewLine > 0 {
			return lines[i].NewLine
		}
	}
	return 1
}

// setDestination chooses where the composer sends comments, and updates the
// hints that tell the user.
func (m *Model) setDestination(d destination) {
	m.destination = d
	agentTarget := m.sender.Target()

	// Only an agent pane can submit a comment. GitHub and the clipboard have nothing to submit to.
	_, submits := m.sender.(agent.Submitter)
	m.composeKeys.SendNow.SetEnabled(d == toAgent && submits)

	if d == toPullRequest {
		m.composer.Placeholder = "Write a review comment for GitHub…"
		m.composeKeys.Send.SetHelp("enter", "comment on GitHub")
		return
	}

	m.composer.Placeholder = "Ask a question about these lines…"
	m.composeKeys.Send.SetHelp("enter", "send to "+agentTarget)
}

// endComment closes the composer and clears the selection.
func (m *Model) endComment() {
	m.mode = browsing
	m.composer.Blur()
	m.anchor = -1
	m.clamp()
}

// setStatus shows a message in the footer for a few seconds.
func (m *Model) setStatus(text string) tea.Cmd {
	m.statusID++
	id := m.statusID
	m.status = text
	return tea.Tick(statusDuration, func(time.Time) tea.Msg { return clearStatusMsg{id} })
}

// contentWidth is the width of the page, after the cursor column.
func (m Model) contentWidth() int {
	return max(1, m.width-1)
}

// bodyHeight is the number of screen lines between the top bar and the footer.
func (m Model) bodyHeight() int {
	return max(1, m.height-1-lipgloss.Height(m.footerView()))
}

// rowAt finds the page row at screen line y, below the top bar. A line of the
// comment box is not a page row.
func (m Model) rowAt(y int) (int, bool) {
	lines := m.screenLines()
	if y < 1 || y-1 >= len(lines) || lines[y-1].row < 0 {
		return 0, false
	}
	return lines[y-1].row, true
}

// screenLine is one line of the body: a page row, or a line of the comment box.
type screenLine struct {
	// row is the page row, or -1 for a line of the comment box.
	row  int
	text string
}

// screenLines lists the body's lines from the top of the screen, with the
// comment box below the last selected line while it is open.
func (m Model) screenLines() []screenLine {
	height := m.bodyHeight()

	var box []string
	boxAfter := -1
	if m.mode == composing {
		box = m.composerBox()
		boxAfter = m.selectionEnd()
	}

	lines := make([]screenLine, 0, height)
	for i := m.offset; i < len(m.rows) && len(lines) < height; i++ {
		lines = append(lines, screenLine{row: i})
		if i != boxAfter {
			continue
		}
		for _, text := range box {
			if len(lines) == height {
				break
			}
			lines = append(lines, screenLine{row: -1, text: text})
		}
	}
	return lines
}

// View draws the screen in the alternate screen, with mouse and focus
// reports on.
func (m Model) View() tea.View {
	view := tea.NewView(m.screen())
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	view.ReportFocus = true
	return view
}

// screen draws the content of the screen.
func (m Model) screen() string {
	switch {
	case m.width == 0:
		return ""
	case m.mode == picking:
		return m.pickerView()
	case m.mode == finding || m.mode == choosingBase:
		return m.finderView()
	case m.review == nil && m.loadErr != nil:
		hint := "r retry · q quit"

		// A branch without a pull request fails to load, and the switch key leads back to the local changes.
		if m.keys.Switch.Enabled() {
			switchHelp := m.keys.Switch.Help()
			hint = "r retry · " + switchHelp.Key + " " + switchHelp.Desc + " · q quit"
		}
		return ansi.Wrap(errorStyle.Render(m.loadErr.Error()), m.width, "") + "\n\n" + dimStyle.Render(hint)
	case m.review == nil:
		return m.spinner.View() + " " + dimStyle.Render("Loading…")
	}

	return strings.Join([]string{m.topBarView(), m.bodyView(), m.footerView()}, "\n")
}

// pickerView lists the worktrees by branch, with tags for the current
// checkout, the main checkout, and a detached HEAD.
func (m Model) pickerView() string {
	title := ansi.Truncate(" Switch worktree", m.width, "…")
	bar := barStyle.Render(title + strings.Repeat(" ", max(0, m.width-ansi.StringWidth(title))))

	// The list scrolls only as far as it must to keep the cursor on the screen.
	height := m.bodyHeight()
	start := max(0, m.pick-height+1)

	lines := make([]string, height)
	for y := range lines {
		i := start + y
		if i >= len(m.worktrees) {
			break
		}
		w := m.worktrees[i]

		var tags []string
		if w.Current {
			tags = append(tags, "current")
		}
		if w.Main {
			tags = append(tags, "main")
		}
		if w.Branch == "" {
			tags = append(tags, "detached")
		}
		tag := strings.Join(tags, " · ")

		marker := " "
		if i == m.pick {
			marker = accentStyle.Render("▌")
		}
		name := ansi.Truncate(w.Name(), max(1, m.contentWidth()-ansi.StringWidth(tag)-3), "…")
		gap := strings.Repeat(" ", max(1, m.contentWidth()-ansi.StringWidth(name)-ansi.StringWidth(tag)-2))
		lines[y] = marker + " " + name + gap + dimStyle.Render(tag)
	}

	return strings.Join([]string{bar, strings.Join(lines, "\n"), m.footerView()}, "\n")
}

// finderView shows the query and the files or branches that match it.
func (m Model) finderView() string {
	title, empty := " Find file", "No matching files."
	if m.mode == choosingBase {
		title, empty = " Compare with branch", "No matching branches."
	}
	title = ansi.Truncate(title, m.width, "…")
	bar := barStyle.Render(title + strings.Repeat(" ", max(0, m.width-ansi.StringWidth(title))))

	finder := m.finder
	finder.SetWidth(max(1, m.contentWidth()-ansi.StringWidth(finder.Prompt)-1))
	input := " " + finder.View()

	// The list scrolls only as far as it must to keep the cursor on the screen.
	items := m.finderItems(m.mode)
	matches := m.finderMatches()
	height := max(1, m.bodyHeight()-1)
	start := max(0, m.selectedMatch-height+1)

	lines := make([]string, height)
	if len(matches) == 0 {
		lines[0] = "  " + dimStyle.Render(empty)
	}
	for y := range lines {
		i := start + y
		if i >= len(matches) {
			break
		}

		marker := " "
		if i == m.selectedMatch {
			marker = accentStyle.Render("▌")
		}
		item := ansi.Truncate(items[matches[i]], max(1, m.contentWidth()-2), "…")
		lines[y] = marker + " " + item
	}

	return strings.Join([]string{bar, input, strings.Join(lines, "\n"), m.footerView()}, "\n")
}

// topBarView shows the file at the top of the screen, or the review's title
// above the diff, and how far down the page the screen is.
func (m Model) topBarView() string {
	context := m.review.Context
	if m.offset < len(m.rows) {
		if file := m.rows[m.offset].file; file >= 0 {
			context = m.files[file].Path
		}
	}

	height := m.bodyHeight()
	percent := 100
	if len(m.rows) > height {
		percent = m.offset * 100 / (len(m.rows) - height)
	}
	position := fmt.Sprintf(" %d%% ", percent)

	left := ansi.Truncate(" "+context, max(0, m.width-ansi.StringWidth(position)), "…")
	gap := strings.Repeat(" ", max(0, m.width-ansi.StringWidth(left)-ansi.StringWidth(position)))
	return barStyle.Render(left+gap) + barDimStyle.Render(position)
}

// bodyView draws the visible page rows with the cursor, the selection, and
// the comment box.
func (m Model) bodyView() string {
	lines := make([]string, m.bodyHeight())
	for y, line := range m.screenLines() {
		if line.row < 0 {
			lines[y] = line.text
			continue
		}
		lines[y] = m.renderRow(line.row)
	}
	return strings.Join(lines, "\n")
}

// renderRow draws one row with the cursor column in front of it.
func (m Model) renderRow(i int) string {
	r := m.rows[i]
	selected := m.isSelected(i)

	marker := " "
	switch {
	case i == m.cursor:
		marker = accentStyle.Render("▌")
	case selected:
		marker = accentStyle.Render("│")
	}

	if r.kind == codeRow {
		return marker + renderCode(r, m.contentWidth(), selected, m.theme)
	}
	return marker + r.text
}

// selectionEnd finds the last selected diff line, where the comment box goes.
func (m Model) selectionEnd() int {
	first, last, _ := m.selection()
	for i := last; i > first; i-- {
		if m.isSelected(i) {
			return i
		}
	}
	return first
}

// composerWidth is the outer width of the comment box.
func (m Model) composerWidth() int {
	return max(8, m.contentWidth()-composerIndent)
}

// composerTextWidth is the width of the text inside the comment box, between
// its borders and padding.
func (m Model) composerTextWidth() int {
	return m.composerWidth() - 4
}

// composerHeight is the number of screen lines the comment box takes, or 0
// when it is closed.
func (m Model) composerHeight() int {
	if m.mode != composing {
		return 0
	}
	return m.composer.Height() + 2
}

// composerBox draws the comment box with the file and lines in its top border.
func (m Model) composerBox() []string {
	width := m.composerWidth()

	// A different color warns that the comment goes to GitHub, where others can read it.
	color := accentColor
	if m.destination == toPullRequest {
		color = mergedColor
	}

	marker := ""
	if m.linesChanged {
		marker = " · lines changed"
	}

	// On a narrow screen, keep the end of the path, which names the file.
	label := fmt.Sprintf("%s %s", m.snippet.Path, m.snippet.Location())
	if room := width - 6 - ansi.StringWidth(marker); ansi.StringWidth(label) > room {
		label = ansi.TruncateLeft(label, ansi.StringWidth(label)-room+1, "…")
	}

	borderStyle := lipgloss.NewStyle().Foreground(color)
	dashes := strings.Repeat("─", max(0, width-ansi.StringWidth(label)-ansi.StringWidth(marker)-5))
	top := borderStyle.Render("╭─ "+label) + errorStyle.Render(marker) + borderStyle.Render(" "+dashes+"╮")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(color).
		Padding(0, 1).
		Width(width)

	lines := append([]string{top}, strings.Split(box.Render(m.composer.View()), "\n")...)

	// The hint sits in the bottom border, next to the text, where the user looks while writing.
	if m.composeKeys.Cycle.Enabled() || m.destination == toPullRequest {
		lines[len(lines)-1] = m.destinationBorder(width, borderStyle)
	}

	// The extra space lines the box up with the cursor column of page rows.
	indent := strings.Repeat(" ", 1+composerIndent)
	for i, line := range lines {
		lines[i] = indent + line
	}
	return lines
}

// destinationBorder draws the bottom border of the comment box with where the
// comment goes and, when it can change, the key that changes it. A narrow box
// drops the key.
func (m Model) destinationBorder(width int, borderStyle lipgloss.Style) string {
	name := "ask agent"
	if m.destination == toPullRequest {
		name = "comment"
	}

	hint := " (shift+tab to cycle)"
	room := width - 5
	if !m.composeKeys.Cycle.Enabled() || ansi.StringWidth(name+hint) > room {
		hint = ""
	}
	name = ansi.Truncate(name, room, "…")

	rest := strings.Repeat("─", max(0, width-ansi.StringWidth(name+hint)-5))
	return borderStyle.Render("╰─ "+name) + dimStyle.Render(hint) + borderStyle.Render(" "+rest+"╯")
}

// footerView shows a status message, or the keys for the current mode.
func (m Model) footerView() string {
	footer := m.help.View(m.keys)
	switch {
	case m.status != "":
		footer = " " + m.status
	case m.mode == composing:
		footer = m.help.View(m.composeKeys)
	case m.mode == picking:
		footer = m.help.View(m.pickKeys)
	case m.mode == finding:
		footer = m.help.View(m.findKeys)
	case m.mode == choosingBase:
		footer = m.help.View(m.baseKeys)
	}

	// Help cuts its own line short, but not on every width, so cut each line here too.
	lines := strings.Split(footer, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "…")
	}

	// The notice wraps instead of being cut, so the update command stays complete in a narrow pane.
	if m.newRelease != nil {
		notice := accentStyle.Render("rexamine "+m.newRelease.Version+" is available:") + " " + dimStyle.Render("`rexamine update`")
		lines = append([]string{ansi.Wrap(" "+notice, m.width, "/")}, lines...)
	}
	return strings.Join(lines, "\n")
}

// loadReview loads the review in the background.
func loadReview(src source.Source, gen int) tea.Cmd {
	return func() tea.Msg {
		review, err := src.Load()
		return loadedMsg{gen, review, err}
	}
}

// waitForTurn waits in the background until the agent starts or ends a turn.
// When the watch ends, it returns no message, so the UI stops waiting.
func waitForTurn(turns <-chan agent.TurnEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-turns
		if !ok {
			return nil
		}
		return turnMsg{event}
	}
}

// takeSnapshot saves the working tree of dir at the start of the agent's turn.
func takeSnapshot(dir string) tea.Cmd {
	return func() tea.Msg {
		tree, err := source.Snapshot(dir)
		return snapshotMsg{tree, err}
	}
}

// listWorktrees lists the checkouts of the repository in dir in the background.
func listWorktrees(dir string) tea.Cmd {
	return func() tea.Msg {
		worktrees, err := source.Worktrees(dir)
		return worktreesMsg{worktrees, err}
	}
}

// listBranches lists the branches of the repository in dir in the background.
func listBranches(dir string) tea.Cmd {
	return func() tea.Msg {
		branches, err := source.Branches(dir)
		return branchesMsg{branches, err}
	}
}

// findCheckout looks for the worktree that has the branch checked out.
func findCheckout(dir, branch string, gen int) tea.Cmd {
	return func() tea.Msg {
		worktrees, err := source.Worktrees(dir)
		msg := checkoutMsg{gen: gen, branch: branch, err: err}
		for _, w := range worktrees {
			if w.Branch == branch {
				msg.worktree, msg.found = w, true
				break
			}
		}
		return msg
	}
}

// send delivers a comment in the background.
func send(sender agent.Sender, text string) tea.Cmd {
	return func() tea.Msg { return sentMsg{sender.Send(text), "Sent to " + sender.Target()} }
}

// submit delivers a comment and submits it in the background.
func submit(submitter agent.Submitter, text string) tea.Cmd {
	return func() tea.Msg {
		return sentMsg{submitter.Submit(text), "Submitted to " + submitter.Target()}
	}
}

// postComment posts a review comment on the pull request in the background.
func postComment(pr github.PullRequest, comment github.ReviewComment) tea.Cmd {
	return func() tea.Msg {
		return sentMsg{github.PostReviewComment(pr, comment), fmt.Sprintf("Commented on #%d", pr.Number)}
	}
}

// reviewComment turns the snippet into a GitHub review comment from its first
// to its last line.
func reviewComment(s agent.Snippet, body string) github.ReviewComment {
	return github.ReviewComment{
		Path:  s.Path,
		Start: commentLine(s.Lines[0]),
		End:   commentLine(s.Lines[len(s.Lines)-1]),
		Body:  body,
	}
}

// commentLine finds where GitHub shows a diff line. A removed line exists only
// in the base, so it has the base's line number.
func commentLine(line diff.Line) github.CommentLine {
	if line.Kind == diff.Removed {
		return github.CommentLine{Number: line.OldLine, Side: github.Left}
	}
	return github.CommentLine{Number: line.NewLine, Side: github.Right}
}

// openInBrowser opens a pull request on GitHub in the background.
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg { return openedMsg{github.OpenInBrowser(url)} }
}

// startEditor opens the file at path under root at line. An editor with its
// own window starts beside rexamine. A terminal editor opens in a new Rex tab,
// so the review stays open. Outside Rex, it takes over the terminal until it quits.
func startEditor(ed editor.Editor, root, path string, line int) tea.Cmd {
	args := ed.Args(filepath.Join(root, path), line)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root

	if !ed.InTerminal() {
		return func() tea.Msg {
			var stderr strings.Builder
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				return openedMsg{fmt.Errorf("open the editor: %w", err)}
			}

			exited := make(chan error, 1)
			go func() { exited <- cmd.Wait() }()

			// A launcher such as `code` hands the file to the app and exits, so a failure shows quickly. An
			// editor that keeps running, such as `code --wait`, is open.
			select {
			case err := <-exited:
				if err != nil {
					return openedMsg{fmt.Errorf("open the editor: %s", cmp.Or(strings.TrimSpace(stderr.String()), err.Error()))}
				}
			case <-time.After(editorStartup):
			}
			return openedMsg{}
		}
	}

	if session := os.Getenv("REX_SESSION"); session != "" {
		return func() tea.Msg {
			rexArgs := append([]string{"run", "-s", session, "-c", root, "--"}, args...)
			if out, err := exec.Command("rex", rexArgs...).CombinedOutput(); err != nil {
				return openedMsg{fmt.Errorf("rex: %s", strings.TrimSpace(string(out)))}
			}
			return openedMsg{}
		}
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			err = fmt.Errorf("open the editor: %w", err)
		}
		return openedMsg{err}
	})
}

// clamp limits n to the range from low to high.
func clamp(n, low, high int) int {
	return max(low, min(n, high))
}
