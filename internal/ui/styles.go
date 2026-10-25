package ui

import "github.com/charmbracelet/lipgloss"

// Colors adapt to light and dark terminals. The diff colors follow GitHub's.
var (
	accentColor    = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"}
	dimColor       = lipgloss.AdaptiveColor{Light: "#6e7781", Dark: "#7d8590"}
	addedColor     = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"}
	removedColor   = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"}
	addedLineBg    = lipgloss.AdaptiveColor{Light: "#e6ffec", Dark: "#12261e"}
	removedLineBg  = lipgloss.AdaptiveColor{Light: "#ffebe9", Dark: "#25171c"}
	warningColor   = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"}
	mergedColor    = lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#a371f7"}
	selectedLineBg = lipgloss.AdaptiveColor{Light: "#ddf4ff", Dark: "#1c2d41"}
	barBg          = lipgloss.AdaptiveColor{Light: "#eaeef2", Dark: "#161b22"}
)

var (
	boldStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(dimColor)
	accentStyle  = lipgloss.NewStyle().Foreground(accentColor)
	addedStyle   = lipgloss.NewStyle().Foreground(addedColor)
	removedStyle = lipgloss.NewStyle().Foreground(removedColor)
	headingStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	hunkStyle    = lipgloss.NewStyle().Foreground(dimColor).Italic(true)
	barStyle     = lipgloss.NewStyle().Background(barBg).Bold(true)
	barDimStyle  = lipgloss.NewStyle().Background(barBg).Foreground(dimColor)
	warningStyle = lipgloss.NewStyle().Foreground(warningColor)
	mergedStyle  = lipgloss.NewStyle().Foreground(mergedColor)
	statusStyle  = lipgloss.NewStyle().Foreground(addedColor)
	errorStyle   = lipgloss.NewStyle().Foreground(removedColor)
)
