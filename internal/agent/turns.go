package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
)

// working is the state that an agent reports to Rex while it does a task.
const working = "working"

// statusRecord is the status that a program reports to Rex with OSC 7501.
// A record without an ID belongs to the program itself, and a record with an
// ID belongs to one of its sub-agents.
type statusRecord struct {
	ID     string `json:"id"`
	Record struct {
		State string `json:"state"`
	} `json:"record"`
}

// statusEvent is one line of `rex events --json` for a status change.
type statusEvent struct {
	Payload struct {
		Event struct {
			Payload struct {
				BlockID string       `json:"block_id"`
				Name    string       `json:"name"`
				Status  statusRecord `json:"payload"`
			} `json:"payload"`
		} `json:"event"`
	} `json:"payload"`
}

// WatchTurns reports each time the agent in the pane starts a turn. It follows
// the status that the agent reports to Rex, such as Claude Code's "working".
// The channel closes when the event stream ends or ctx is canceled.
func (p RexPane) WatchTurns(ctx context.Context) (<-chan struct{}, error) {
	cmd := exec.CommandContext(ctx, "rex", "events", "-s", p.Session, "--json")
	events, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// A turn that already runs has no snapshot from its start, so only the next turn counts.
	busy := p.isWorking()

	turns := make(chan struct{}, 1)
	go func() {
		defer close(turns)
		watchTurns(events, p.Block, busy, turns)
		_ = cmd.Wait()
	}()
	return turns, nil
}

// isWorking checks whether the agent in the pane is in the middle of a turn.
func (p RexPane) isWorking() bool {
	out, err := exec.Command("rex", "block", "call", "-s", p.Session, "-b", p.Block, "program_status").Output()
	if err != nil {
		return false
	}

	var status struct {
		Records []struct {
			ID    string `json:"id"`
			State string `json:"state"`
		} `json:"records"`
	}
	if json.Unmarshal(out, &status) != nil {
		return false
	}
	for _, record := range status.Records {
		if record.ID == "" {
			return record.State == working
		}
	}
	return false
}

// watchTurns reads Rex events and sends on turns when the agent in block
// starts to work. The agent repeats "working" with a new message as it
// progresses, so only the change into "working" starts a turn.
func watchTurns(events io.Reader, block string, busy bool, turns chan<- struct{}) {
	lines := bufio.NewScanner(events)
	lines.Buffer(nil, 1<<20)
	for lines.Scan() {
		var event statusEvent
		if json.Unmarshal(lines.Bytes(), &event) != nil {
			continue
		}

		change := event.Payload.Event.Payload
		if change.BlockID != block || change.Name != "program_status_changed" || change.Status.ID != "" {
			continue
		}

		started := change.Status.Record.State == working && !busy
		busy = change.Status.Record.State == working
		if !started {
			continue
		}

		// The review needs only the latest turn, so a turn that the UI did not take yet is enough.
		select {
		case turns <- struct{}{}:
		default:
		}
	}
}
