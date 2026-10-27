package ui

import (
	"image/color"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// The palette indexes of the terminal theme's red and green.
const (
	paletteRed   = 1
	paletteGreen = 2
)

// terminalTheme holds the colors that the terminal reports, and the line
// backgrounds of changes. The line backgrounds keep the GitHub tints until
// the terminal reports its background, red, and green.
type terminalTheme struct {
	background, red, green     color.Color
	darkBackground             bool
	addedLineBg, removedLineBg color.Color
}

// newTerminalTheme creates a theme with the GitHub line backgrounds.
func newTerminalTheme() terminalTheme {
	return terminalTheme{addedLineBg: addedLineBg, removedLineBg: removedLineBg}
}

// requestTerminalTheme asks the terminal for its background, red, and green.
// The replies come back as messages for withReply.
func requestTerminalTheme() tea.Cmd {
	paletteQuery := "\x1b]4;" + strconv.Itoa(paletteRed) + ";?\x07\x1b]4;" + strconv.Itoa(paletteGreen) + ";?\x07"
	return tea.Batch(tea.RequestBackgroundColor, tea.Raw(paletteQuery))
}

// withReply stores a color that the terminal reports. Once the background,
// red, and green are all known, it mixes the line backgrounds from them.
func (t terminalTheme) withReply(msg tea.Msg) terminalTheme {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		t.background = msg.Color
		t.darkBackground = msg.IsDark()

	// Bubble Tea has no message for palette colors, so their replies arrive unparsed.
	case uv.UnknownOscEvent:
		index, reported, ok := paletteColor(string(msg))
		switch {
		case ok && index == paletteRed:
			t.red = reported
		case ok && index == paletteGreen:
			t.green = reported
		}
	}

	if t.background == nil || t.red == nil || t.green == nil {
		return t
	}
	t.addedLineBg = t.tint(t.green)
	t.removedLineBg = t.tint(t.red)
	return t
}

// paletteColor reads a reply to a palette color query, such as
// "\x1b]4;2;rgb:0000/d8d8/9494\x07", and returns its index and color.
func paletteColor(reply string) (int, color.Color, bool) {
	body, isPalette := strings.CutPrefix(reply, "\x1b]4;")
	if !isPalette {
		return 0, nil, false
	}

	// Terminals end the reply with BEL or with ESC \, depending on how the query ended.
	body = strings.TrimSuffix(strings.TrimSuffix(body, "\x07"), "\x1b\\")
	index, spec, ok := strings.Cut(body, ";")
	if !ok {
		return 0, nil, false
	}

	n, err := strconv.Atoi(index)
	reported := ansi.XParseColor(spec)
	if err != nil || reported == nil {
		return 0, nil, false
	}
	return n, reported, true
}

// tint mixes one sixth of a theme color into the background, which gives a
// soft line background that matches the theme. On a dark background, the
// line is also darker than the page, so its text stands out more.
func (t terminalTheme) tint(themeColor color.Color) color.Color {
	base := t.background
	if t.darkBackground {
		base = lipgloss.Darken(base, 0.6)
	}
	return lipgloss.Blend1D(7, base, themeColor)[1]
}
