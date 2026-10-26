package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/segersniels/rexdiff/internal/diff"
	"github.com/segersniels/rexdiff/internal/github"
	"github.com/segersniels/rexdiff/internal/source"
)

// newMarkdownRenderer creates a markdown renderer that wraps to width and
// starts at the left edge, in line with the rest of the page.
func newMarkdownRenderer(width int, markdownStyle string) (*glamour.TermRenderer, error) {
	style, ok := styles.DefaultStyles[markdownStyle]
	if !ok {
		style = &styles.DarkStyleConfig
	}

	flush := *style
	noMargin := uint(0)
	flush.Document.Margin = &noMargin

	return glamour.NewTermRenderer(glamour.WithStyles(flush), glamour.WithWordWrap(width))
}

// renderHeader renders what the review is, the branches it compares, and
// its size. A pull request also shows its state, author, review status,
// reviewers, and labels.
func renderHeader(review source.Review, files []diff.File, width int) string {
	pr := review.PullRequest

	context := dimStyle.Render(review.Context)
	if pr != nil {
		context += "  " + stateBadge(*pr)
	}
	lines := []string{context, boldStyle.Render(review.Title)}

	if pr != nil {
		lines = append(lines, pr.Author.Login+dimStyle.Render(" · updated "+timeAgo(pr.UpdatedAt)))
	}
	lines = append(lines, accentStyle.Render(review.Base)+dimStyle.Render(" ← "+review.Head))

	added, removed := 0, 0
	for _, file := range files {
		a, r := file.Stats()
		added += a
		removed += r
	}
	fileCount := fmt.Sprintf("%d files", len(files))
	if len(files) == 1 {
		fileCount = "1 file"
	}
	stats := dimStyle.Render(fileCount) + "  " +
		addedStyle.Render(fmt.Sprintf("+%d", added)) + " " + removedStyle.Render(fmt.Sprintf("−%d", removed))
	if pr != nil {
		if badge := reviewBadge(pr.ReviewDecision); badge != "" {
			stats += "  " + badge
		}
	}
	lines = append(lines, stats)

	for i, line := range lines {
		lines[i] = ansi.Wrap(line, width, " /")
	}

	if pr != nil {
		if chips := reviewerChips(*pr); len(chips) > 0 {
			lines = append(lines, flow(append([]string{dimStyle.Render("Reviewers")}, chips...), width)...)
		}
	}

	if pr != nil && len(pr.Labels) > 0 {
		chips := make([]string, len(pr.Labels))
		for i, label := range pr.Labels {
			chips[i] = labelChip(label)
		}
		lines = append(lines, flow(chips, width)...)
	}
	return strings.Join(lines, "\n")
}

// stateBadge renders the pull request's state in GitHub's colors.
func stateBadge(pr github.PullRequest) string {
	switch {
	case pr.IsDraft:
		return dimStyle.Render("● Draft")
	case pr.State == "MERGED":
		return mergedStyle.Render("● Merged")
	case pr.State == "CLOSED":
		return removedStyle.Render("● Closed")
	default:
		return addedStyle.Render("● Open")
	}
}

// reviewBadge renders the review decision, or nothing when no review is required.
func reviewBadge(decision string) string {
	switch decision {
	case "APPROVED":
		return addedStyle.Render("✓ Approved")
	case "CHANGES_REQUESTED":
		return removedStyle.Render("✗ Changes requested")
	case "REVIEW_REQUIRED":
		return warningStyle.Render("● Review required")
	default:
		return ""
	}
}

// reviewerChips renders each reviewer with the state of their latest review,
// followed by the requested reviewers who have not reviewed yet.
func reviewerChips(pr github.PullRequest) []string {
	var chips []string
	reviewed := map[string]bool{}
	for _, review := range pr.LatestReviews {
		login := review.Author.Login

		// The author's replies to review threads count as reviews, but the author does not review.
		if login == pr.Author.Login {
			continue
		}

		switch review.State {
		case "APPROVED":
			chips = append(chips, addedStyle.Render("✓ "+login))
		case "CHANGES_REQUESTED":
			chips = append(chips, removedStyle.Render("✗ "+login))
		case "COMMENTED":
			chips = append(chips, dimStyle.Render("○ "+login))
		default:
			continue
		}
		reviewed[login] = true
	}

	for _, request := range pr.ReviewRequests {
		name := request.Login
		if name == "" {
			name = request.Name
		}
		if reviewed[name] {
			continue
		}
		chips = append(chips, warningStyle.Render("● ")+name)
	}
	return chips
}

// labelChip renders a label in its GitHub color, with text that stays
// readable on that color.
func labelChip(label github.Label) string {
	if len(label.Color) != 6 {
		return dimStyle.Render("[" + label.Name + "]")
	}

	var r, g, b int
	if _, err := fmt.Sscanf(label.Color, "%02x%02x%02x", &r, &g, &b); err != nil {
		return dimStyle.Render("[" + label.Name + "]")
	}

	// Dark text on light colors, light text on dark colors, by perceived brightness.
	text := lipgloss.Color("#ffffff")
	if (r*299+g*587+b*114)/1000 > 150 {
		text = lipgloss.Color("#000000")
	}
	return lipgloss.NewStyle().Background(lipgloss.Color("#" + label.Color)).Foreground(text).Render(" " + label.Name + " ")
}

// flow places items on lines of at most width, with a space between items
// and new lines where they no longer fit. An item wider than a line is cut.
func flow(items []string, width int) []string {
	var lines []string
	line := ""
	for _, item := range items {
		item = ansi.Truncate(item, width, "…")
		switch {
		case line == "":
			line = item
		case ansi.StringWidth(line)+1+ansi.StringWidth(item) <= width:
			line += " " + item
		default:
			lines = append(lines, line)
			line = item
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// timeAgo describes how long ago a time was, such as "3h ago".
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2, 2006")
	}
}

// renderHeading renders a section heading with a rule that fills the width.
func renderHeading(title string, width int) string {
	rule := strings.Repeat("─", max(0, width-ansi.StringWidth(title)-1))
	return ansi.Truncate(headingStyle.Render(title)+" "+dimStyle.Render(rule), width, "")
}

// renderMarkdown renders GitHub markdown, or a placeholder when it is empty.
func renderMarkdown(markdown *glamour.TermRenderer, body string, width int) string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" {
		return dimStyle.Render("No description.")
	}

	out, err := markdown.Render(body)
	if err != nil {
		return ansi.Wrap(body, width, "")
	}

	// Glamour can overflow on long words, such as URLs, so cut those lines to fit.
	lines := strings.Split(strings.Trim(out, "\n"), "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}
