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

// Submitter is a Sender that can also submit the text, so the agent starts on
// it at once instead of waiting for the user.
type Submitter interface {
	Sender
	Submit(text string) error
}

// RexPane pastes text into a Rex terminal pane, such as one that runs Claude
// Code. The agent receives a real paste, so the text waits in its prompt
// until the user submits it.
type RexPane struct {
	Session string
	Block   string
}

// Send pastes the text into the pane through the rex CLI. When rex fails, it
// copies the text to the clipboard, so the comment is not lost.
func (p RexPane) Send(text string) error {
	err := p.paste(text)
	if err == nil {
		return nil
	}

	// Rex is in beta, so an update can change the API that the paste uses.
	if clipErr := clipboard.WriteAll(text); clipErr != nil {
		return err
	}
	return fmt.Errorf("%w; copied the comment to the clipboard instead", err)
}

// paste sends the text to the pane as a paste event.
func (p RexPane) paste(text string) error {
	request, err := json.Marshal(map[string]any{
		"block_id": p.Block,
		"event":    map[string]any{"type": "paste", "payload": map[string]string{"text": text}},
	})
	if err != nil {
		return err
	}

	cmd := p.rex("api", "call", "block.event", "-")
	cmd.Stdin = bytes.NewReader(request)
	return run(cmd)
}

// Submit pastes the text into the pane, presses enter to submit it, and
// focuses the pane, so the user sees the agent answer.
func (p RexPane) Submit(text string) error {
	if err := p.paste(text); err != nil {
		return err
	}
	if err := run(p.rex("send-key", "-b", p.Block, "enter")); err != nil {
		return err
	}
	return run(p.rex("focus", p.Block))
}

// rex builds a rex command that acts on the pane's session.
func (p RexPane) rex(args ...string) *exec.Cmd {
	if p.Session != "" {
		args = append(args, "-s", p.Session)
	}
	return exec.Command("rex", args...)
}

// run runs a rex command and returns its output as the error when it fails.
func run(cmd *exec.Cmd) error {
	if out, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(out))

		// rex prints nothing when it cannot start, for example when it is not installed.
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("rex: %s", message)
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
