package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestPaletteColorReadsBothReplyEndings(t *testing.T) {
	for _, reply := range []string{"\x1b]4;2;rgb:0000/d8d8/9494\x07", "\x1b]4;2;rgb:0000/d8d8/9494\x1b\\"} {
		index, reported, ok := paletteColor(reply)
		if !ok || index != 2 {
			t.Fatalf("paletteColor(%q) = %d, %v, want index 2", reply, index, ok)
		}
		if r, g, b, _ := reported.RGBA(); r>>8 != 0x00 || g>>8 != 0xd8 || b>>8 != 0x94 {
			t.Errorf("paletteColor(%q) color = %02x%02x%02x, want 00d894", reply, r>>8, g>>8, b>>8)
		}
	}

	if _, _, ok := paletteColor("\x1b]11;rgb:3838/3838/3838\x07"); ok {
		t.Error("paletteColor read a background reply as a palette color")
	}
}

func TestThemeKeepsGitHubTintsUntilAllColorsArrive(t *testing.T) {
	theme := newTerminalTheme()
	theme = theme.withReply(tea.BackgroundColorMsg{Color: lipgloss.Color("#383838")})
	theme = theme.withReply(uv.UnknownOscEvent("\x1b]4;1;rgb:ebeb/6f6f/9292\x07"))
	if theme.addedLineBg != addedLineBg {
		t.Fatal("the line backgrounds changed before the terminal reported its green")
	}

	theme = theme.withReply(uv.UnknownOscEvent("\x1b]4;2;rgb:0000/d8d8/9494\x07"))
	if theme.addedLineBg == addedLineBg || theme.removedLineBg == removedLineBg {
		t.Error("the line backgrounds did not change after the terminal reported all its colors")
	}
}
