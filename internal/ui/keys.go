package ui

import "charm.land/bubbles/v2/key"

// browseKeys holds the keys for reading the page and selecting lines.
type browseKeys struct {
	Up, Down, HalfUp, HalfDown, PageUp, PageDown key.Binding
	Top, Bottom, NextFile, PrevFile, Collapse    key.Binding
	FindFile, HideTests                          key.Binding
	Select, Comment, Clear                       key.Binding
	Toggle, Worktree, Switch, Open, Reload       key.Binding
	Help, Quit                                   key.Binding
}

func newBrowseKeys() browseKeys {
	return browseKeys{
		Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		HalfUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half page up")),
		HalfDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half page down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "f", " "), key.WithHelp("pgdn/f", "page down")),
		Top:      key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "end")),
		NextFile: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next file")),
		PrevFile: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "previous file")),
		FindFile: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "find file")),
		Collapse: key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "collapse/expand")),
		Select:   key.NewBinding(key.WithKeys("v", "V"), key.WithHelp("v", "select")),
		Comment:  key.NewBinding(key.WithKeys("c", "enter"), key.WithHelp("c", "comment")),
		Clear:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear selection")),
		Toggle:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "scope")),
		Worktree: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "worktree")),
		Switch:   key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull request/local")),
		Open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open on GitHub")),
		Reload:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),

		HideTests: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "hide/show tests")),
	}
}

// ShortHelp lists the keys for the one-line footer.
func (k browseKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Comment, k.Toggle, k.Worktree, k.Switch, k.Open, k.Help, k.Quit}
}

// FullHelp lists every key, grouped in columns.
func (k browseKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.HalfUp, k.HalfDown, k.PageUp, k.PageDown},
		{k.Top, k.Bottom, k.NextFile, k.PrevFile, k.FindFile, k.Collapse, k.HideTests},
		{k.Select, k.Comment, k.Clear},
		{k.Toggle, k.Worktree, k.Switch, k.Open, k.Reload, k.Help, k.Quit},
	}
}

// composeKeys holds the keys for writing a comment.
type composeKeys struct {
	Send, SendNow, Cycle, Newline, Cancel key.Binding
}

// newComposeKeys creates the composer keys. The model sets the help of Send,
// because it depends on where the comment goes. The comment box shows the
// Cycle key in its border, so the footer leaves it out.
func newComposeKeys() composeKeys {
	return composeKeys{
		Send:    key.NewBinding(key.WithKeys("enter")),
		SendNow: key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send and submit")),
		Cycle:   key.NewBinding(key.WithKeys("shift+tab")),
		Newline: key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"), key.WithHelp("alt+enter", "new line")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// ShortHelp lists the composer keys for the footer.
func (k composeKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.SendNow, k.Newline, k.Cancel}
}

// FullHelp lists the composer keys in one column.
func (k composeKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// pickKeys holds the keys for choosing a worktree.
type pickKeys struct {
	Up, Down, Switch, Cancel key.Binding
}

func newPickKeys() pickKeys {
	return pickKeys{
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Switch: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "switch")),
		Cancel: key.NewBinding(key.WithKeys("esc", "q", "w"), key.WithHelp("esc", "cancel")),
	}
}

// ShortHelp lists the picker keys for the footer.
func (k pickKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Switch, k.Cancel}
}

// FullHelp lists the picker keys in one column.
func (k pickKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// findKeys holds the keys for finding a file. Letters go to the query, so the
// keys to move are arrows and control keys.
type findKeys struct {
	Up, Down, Jump, Cancel key.Binding
}

func newFindKeys() findKeys {
	return findKeys{
		Up:     key.NewBinding(key.WithKeys("up", "ctrl+p"), key.WithHelp("↑/ctrl+p", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "ctrl+n"), key.WithHelp("↓/ctrl+n", "down")),
		Jump:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "go to file")),
		Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// ShortHelp lists the finder keys for the footer.
func (k findKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Jump, k.Cancel}
}

// FullHelp lists the finder keys in one column.
func (k findKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
