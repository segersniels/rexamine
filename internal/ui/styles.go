package ui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

// Colors adapt to light and dark terminals. The diff colors follow GitHub's.
var (
	accentColor    = compat.AdaptiveColor{Light: lipgloss.Color("#0969da"), Dark: lipgloss.Color("#58a6ff")}
	dimColor       = compat.AdaptiveColor{Light: lipgloss.Color("#6e7781"), Dark: lipgloss.Color("#7d8590")}
	addedColor     = compat.AdaptiveColor{Light: lipgloss.Color("#1a7f37"), Dark: lipgloss.Color("#3fb950")}
	removedColor   = compat.AdaptiveColor{Light: lipgloss.Color("#cf222e"), Dark: lipgloss.Color("#f85149")}
	addedLineBg    = compat.AdaptiveColor{Light: lipgloss.Color("#e6ffec"), Dark: lipgloss.Color("#12261e")}
	removedLineBg  = compat.AdaptiveColor{Light: lipgloss.Color("#ffebe9"), Dark: lipgloss.Color("#25171c")}
	warningColor   = compat.AdaptiveColor{Light: lipgloss.Color("#9a6700"), Dark: lipgloss.Color("#d29922")}
	mergedColor    = compat.AdaptiveColor{Light: lipgloss.Color("#8250df"), Dark: lipgloss.Color("#a371f7")}
	selectedLineBg = compat.AdaptiveColor{Light: lipgloss.Color("#ddf4ff"), Dark: lipgloss.Color("#1c2d41")}
	barBg          = compat.AdaptiveColor{Light: lipgloss.Color("#eaeef2"), Dark: lipgloss.Color("#161b22")}
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
