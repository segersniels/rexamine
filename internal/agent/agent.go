// Package agent turns review comments into prompt text and delivers it to a
// coding agent.
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"

	"github.com/segersniels/rexdiff/internal/diff"
)

// Snippet is a range of diff lines in one file that a comment is about.
type Snippet struct {
	Path  string
	Lines []diff.Line
}

// Location describes where the snippet is, such as "L2-L5". It uses the new
// file's line numbers, unless every line in the snippet was removed.
func (s Snippet) Location() string {
	var numbers []int
	for _, line := range s.Lines {
		if line.Kind != diff.Removed {
			numbers = append(numbers, line.NewLine)
		}
	}

	suffix := ""
	if len(numbers) == 0 {
		suffix = " (removed)"
		for _, line := range s.Lines {
			numbers = append(numbers, line.OldLine)
		}
	}
	if len(numbers) == 0 {
		return ""
	}

	first, last := numbers[0], numbers[len(numbers)-1]
	if first == last {
		return fmt.Sprintf("L%d%s", first, suffix)
	}
	return fmt.Sprintf("L%d-L%d%s", first, last, suffix)
}

// Format builds the prompt text for a comment: the location, the selected
// lines as a diff block, and the comment. The text ends with a blank line, so
// several comments in one prompt stay apart.
func Format(s Snippet, comment string) string {
	var code strings.Builder
	for _, line := range s.Lines {
		code.WriteString(line.Kind.Sign() + line.Text + "\n")
	}

	// A longer fence keeps code that contains ``` from closing the block.
	fence := "```"
	for strings.Contains(code.String(), fence) {
		fence += "`"
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, "%s %s:\n%sdiff\n%s%s\n", s.Path, s.Location(), fence, code.String(), fence)
	if comment = strings.TrimSpace(comment); comment != "" {
		prompt.WriteString(comment + "\n")
	}
	prompt.WriteString("\n")
	return prompt.String()
}

// Sender delivers prompt text to the agent.
type Sender interface {
	Send(text string) error

	// Target names where Send puts the text, such as "agent", for messages.
	Target() string
}

// RexPane pastes text into a Rex terminal pane, such as one that runs Claude
// Code. The agent receives a real paste, so the text waits in its prompt
// until the user submits it.
type RexPane struct {
	Session string
	Block   string
}

// Send pastes the text into the pane through the rex CLI.
func (p RexPane) Send(text string) error {
	request, err := json.Marshal(map[string]any{
		"block_id": p.Block,
		"event":    map[string]any{"type": "paste", "payload": map[string]string{"text": text}},
	})
	if err != nil {
		return err
	}

	args := []string{"api", "call"}
	if p.Session != "" {
		args = append(args, "-s", p.Session)
	}
	args = append(args, "block.event", "-")

	cmd := exec.Command("rex", args...)
	cmd.Stdin = bytes.NewReader(request)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rex: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Target returns "agent".
func (RexPane) Target() string { return "agent" }

// Clipboard copies text to the system clipboard, so the user can paste it
// into any agent.
type Clipboard struct{}

// Send copies the text to the clipboard.
func (Clipboard) Send(text string) error { return clipboard.WriteAll(text) }

// Target returns "clipboard".
func (Clipboard) Target() string { return "clipboard" }
