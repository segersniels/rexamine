package ui

import "github.com/charmbracelet/bubbles/key"

// browseKeys holds the keys for reading the page and selecting lines.
type browseKeys struct {
	Up, Down, HalfUp, HalfDown, PageUp, PageDown key.Binding
	Top, Bottom, NextFile, PrevFile, Collapse    key.Binding
	Select, Comment, Clear                       key.Binding
	Toggle, Open, Reload, Help, Quit             key.Binding
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
		Collapse: key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "collapse/expand file")),
		Select:   key.NewBinding(key.WithKeys("v", "V"), key.WithHelp("v", "select")),
		Comment:  key.NewBinding(key.WithKeys("c", "enter"), key.WithHelp("c", "comment")),
		Clear:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear selection")),
		Toggle:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "branch/uncommitted")),
		Open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open on GitHub")),
		Reload:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp lists the keys for the one-line footer.
func (k browseKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Comment, k.NextFile, k.Toggle, k.Help, k.Quit}
}

// FullHelp lists every key, grouped in columns.
func (k browseKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.HalfUp, k.HalfDown, k.PageUp, k.PageDown},
		{k.Top, k.Bottom, k.NextFile, k.PrevFile, k.Collapse},
		{k.Select, k.Comment, k.Clear},
		{k.Toggle, k.Open, k.Reload, k.Help, k.Quit},
	}
}

// composeKeys holds the keys for writing a comment.
type composeKeys struct {
	Send, Newline, Cancel key.Binding
}

func newComposeKeys(target string) composeKeys {
	return composeKeys{
		Send:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "send to "+target)),
		Newline: key.NewBinding(key.WithKeys("alt+enter", "ctrl+j"), key.WithHelp("alt+enter", "new line")),
		Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	}
}

// ShortHelp lists the composer keys for the footer.
func (k composeKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.Newline, k.Cancel}
}

// FullHelp lists the composer keys in one column.
func (k composeKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}
