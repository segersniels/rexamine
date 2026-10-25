// Package ui is the review screen of rexdiff: a scrollable page with the
// review's details and its diff, a line cursor, line selection, and a
// composer that sends comments on the selected lines to an agent.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexdiff/internal/agent"
	"github.com/segersniels/rexdiff/internal/diff"
	"github.com/segersniels/rexdiff/internal/github"
	"github.com/segersniels/rexdiff/internal/source"
	"github.com/segersniels/rexdiff/internal/update"
)

// watchInterval is how often a local review checks the repository for changes.
const watchInterval = 2 * time.Second

// statusDuration is how long a status message stays in the footer.
const statusDuration = 3 * time.Second

// composerIndent is how far the comment box is indented under the code.
const composerIndent = 2

// mode tells whether keys go to the page or to the comment composer.
type mode int

const (
	browsing mode = iota
	composing
)

type (
	loadedMsg struct {
		gen    int
		review source.Review
		err    error
	}
	watchMsg       struct{ gen int }
	turnStartedMsg struct{}
	snapshotMsg    struct {
		tree string
		err  error
	}
	sentMsg        struct{ err error }
	openedMsg      struct{ err error }
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
	source        source.Source
	sender        agent.Sender
	markdownStyle string

	// turns reports when the agent starts a turn, or is nil without an agent to watch.
	turns <-chan struct{}

	keys        browseKeys
	composeKeys composeKeys
	help        help.Model
	spinner     spinner.Model
	composer    textarea.Model

	review  *source.Review
	files   []diff.File
	loadErr error

	// highlights holds the syntax colors of files. They are computed once per
	// load, because a new layout on each resize must stay fast.
	highlights []highlightedFile

	loading  bool
	loadGen  int
	watchGen int

	width, height int
	rows          []row
	fileStarts    []int

	// collapsed holds the paths of the collapsed files. Paths stay the same
	// across reloads, so a file stays collapsed when the diff changes.
	collapsed map[string]bool

	offset int
	cursor int

	// anchor is the row where the selection starts, or -1 without a selection.
	anchor   int
	dragging bool

	// pressedGutter is set when a mouse press started on a line number.
	pressedGutter bool
	mode          mode
	snippet       agent.Snippet

	status   string
	statusID int

	// newRelease tells about a newer release of rexdiff, or is nil.
	newRelease *update.Notice
}

// New creates the review screen. Comments go to the sender. turns reports when
// the agent starts a turn, or is nil without an agent to watch. markdownStyle
// is a glamour style name, such as "dark".
func New(src source.Source, sender agent.Sender, turns <-chan struct{}, markdownStyle string) Model {
	composer := textarea.New()
	composer.Placeholder = "Ask a question about these lines…"
	composer.ShowLineNumbers = false
	composer.Prompt = ""
	composer.CharLimit = 0
	composer.SetHeight(3)
	composer.KeyMap.InsertNewline.SetKeys("alt+enter", "ctrl+j")
	composer.FocusedStyle.CursorLine = lipgloss.NewStyle()

	_, local := src.(source.Local)
	keys := newBrowseKeys()
	keys.Toggle.SetEnabled(local)
	keys.Open.SetEnabled(!local)
	if turns != nil {
		keys.Toggle.SetHelp("t", "branch/uncommitted/last turn")
	}

	return Model{
		source:        src,
		sender:        sender,
		markdownStyle: markdownStyle,
		turns:         turns,
		keys:          keys,
		composeKeys:   newComposeKeys(sender.Target()),
		help:          help.New(),
		spinner:       spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(accentStyle)),
		composer:      composer,
		collapsed:     map[string]bool{},
		loading:       true,
		loadGen:       1,
		anchor:        -1,
	}
}

// Init starts the first load.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadReview(m.source, m.loadGen), m.spinner.Tick}
	if m.turns != nil {
		cmds = append(cmds, waitForTurn(m.turns))
	}
	return tea.Batch(cmds...)
}

// Update handles a message and returns the next state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		m.composer.SetWidth(m.composerTextWidth())
		m.layout(m.files)
		return m, nil

	case loadedMsg:
		return m.handleLoaded(msg)

	case watchMsg:
		if msg.gen != m.watchGen {
			return m, nil
		}
		return m, m.reload()

	case turnStartedMsg:
		return m, tea.Batch(takeSnapshot, waitForTurn(m.turns))

	case snapshotMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render("Save the start of the agent's turn: " + msg.err.Error()))
		}
		local := m.source.(source.Local)

		// The turn that just ended changed files, so its start becomes the fallback for a turn without edits.
		if local.TurnStart != "" && msg.tree != local.TurnStart {
			local.PreviousTurnStart = local.TurnStart
		}
		local.TurnStart = msg.tree
		m.source = local
		if local.Scope != source.LastTurn {
			return m, nil
		}
		return m, m.reload()

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
		return m, m.setStatus(statusStyle.Render("✓ Sent to " + m.sender.Target()))

	case openedMsg:
		if msg.err != nil {
			return m, m.setStatus(errorStyle.Render(msg.err.Error()))
		}
		return m, nil

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

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.mode == composing {
			return m.updateComposer(msg)
		}
		return m.updateBrowser(msg)

	case tea.MouseMsg:
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
	unchanged := m.review != nil && msg.review.PullRequest == nil &&
		msg.review.Title == m.review.Title && msg.review.Diff == m.review.Diff
	if unchanged {
		return m, watch
	}

	m.loadErr = nil
	m.review = &msg.review
	files := diff.Parse(msg.review.Diff)
	m.highlights = highlightFiles(files, m.markdownStyle)
	m.layout(files)
	return m, watch
}

// updateBrowser handles keys while the user reads the page.
func (m Model) updateBrowser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case key.Matches(msg, m.keys.Collapse):
		m.toggleFile(m.cursor)
	case key.Matches(msg, m.keys.Select):
		return m, m.toggleSelection()
	case key.Matches(msg, m.keys.Clear):
		m.anchor = -1
	case key.Matches(msg, m.keys.Comment):
		return m, m.startComment()
	case key.Matches(msg, m.keys.Toggle):
		local := m.source.(source.Local)
		local.Scope = local.Scope.Next()

		// Without an agent to watch, there are no turns to show.
		if local.Scope == source.LastTurn && m.turns == nil {
			local.Scope = local.Scope.Next()
		}
		m.source = local
		return m, tea.Batch(m.reload(), m.setStatus(dimStyle.Render("Showing "+local.Scope.String()+"…")))
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

// updateComposer handles keys while the user writes a comment.
func (m Model) updateComposer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
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
	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		m.scrollBy(-3)
	case msg.Button == tea.MouseButtonWheelDown:
		m.scrollBy(3)

	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		i, ok := m.rowAt(msg.Y)
		if !ok {
			return nil
		}

		// A click on the page closes an open comment box without sending it.
		if m.mode == composing {
			m.endComment()
		}

		// A click on a file title collapses or expands the file.
		if r := m.rows[i]; r.kind == fileRow && r.hunk == -1 {
			m.toggleFile(i)
			return nil
		}

		m.cursor = i
		m.anchor = -1
		m.dragging = m.rows[i].kind == codeRow
		m.pressedGutter = m.dragging && msg.X >= 1 && msg.X <= ansi.StringWidth(m.rows[i].gutter)
		if m.dragging {
			m.anchor = i
		}

	case msg.Action == tea.MouseActionMotion && m.dragging:
		m.cursor = m.dragTarget(msg.Y)
		m.clamp()

	case msg.Action == tea.MouseActionRelease && m.dragging:
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
	return loadReview(m.source, m.loadGen)
}

// scheduleWatch plans the next check for local changes. Pull requests change
// rarely, so they reload only on request.
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
	m.rows, m.fileStarts = layoutPage(*m.review, files, m.highlights, m.contentWidth(), m.markdownStyle, m.collapsed)

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
	m.mode = composing
	m.composer.Reset()
	m.composer.SetWidth(m.composerTextWidth())
	m.clamp()
	return m.composer.Focus()
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

// View draws the screen.
func (m Model) View() string {
	switch {
	case m.width == 0:
		return ""
	case m.review == nil && m.loadErr != nil:
		return ansi.Wrap(errorStyle.Render(m.loadErr.Error()), m.width, "") + "\n\n" + dimStyle.Render("r retry · q quit")
	case m.review == nil:
		return m.spinner.View() + " " + dimStyle.Render("Loading…")
	}

	return strings.Join([]string{m.topBarView(), m.bodyView(), m.footerView()}, "\n")
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
		return marker + renderCode(r, m.contentWidth(), selected)
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

	// On a narrow screen, keep the end of the path, which names the file.
	label := fmt.Sprintf("%s %s", m.snippet.Path, m.snippet.Location())
	if room := width - 6; ansi.StringWidth(label) > room {
		label = ansi.TruncateLeft(label, ansi.StringWidth(label)-room+1, "…")
	}
	top := "╭─ " + label + " " + strings.Repeat("─", max(0, width-ansi.StringWidth(label)-5)) + "╮"

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderTop(false).
		BorderForeground(accentColor).
		Padding(0, 1).
		Width(width - 2)

	lines := append([]string{accentStyle.Render(top)}, strings.Split(box.Render(m.composer.View()), "\n")...)

	// The extra space lines the box up with the cursor column of page rows.
	indent := strings.Repeat(" ", 1+composerIndent)
	for i, line := range lines {
		lines[i] = indent + line
	}
	return lines
}

// footerView shows a status message, or the keys for the current mode.
func (m Model) footerView() string {
	footer := m.help.View(m.keys)
	switch {
	case m.status != "":
		footer = " " + m.status
	case m.mode == composing:
		footer = m.help.View(m.composeKeys)
	}

	// Help cuts its own line short, but not on every width, so cut each line here too.
	lines := strings.Split(footer, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "…")
	}

	// The notice wraps instead of being cut, so the update command stays complete in a narrow pane.
	if m.newRelease != nil {
		notice := accentStyle.Render("rexdiff "+m.newRelease.Version+" is available:") + " " + dimStyle.Render(m.newRelease.Command)
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

// waitForTurn waits in the background until the agent starts a turn. When the
// watch ends, it returns no message, so the UI stops waiting.
func waitForTurn(turns <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-turns; !ok {
			return nil
		}
		return turnStartedMsg{}
	}
}

// takeSnapshot saves the working tree at the start of the agent's turn.
func takeSnapshot() tea.Msg {
	tree, err := source.Snapshot()
	return snapshotMsg{tree, err}
}

// send delivers a comment in the background.
func send(sender agent.Sender, text string) tea.Cmd {
	return func() tea.Msg { return sentMsg{sender.Send(text)} }
}

// openInBrowser opens a pull request on GitHub in the background.
func openInBrowser(url string) tea.Cmd {
	return func() tea.Msg { return openedMsg{github.OpenInBrowser(url)} }
}

// clamp limits n to the range from low to high.
func clamp(n, low, high int) int {
	return max(low, min(n, high))
}
