package agent

import (
	"fmt"
	"strings"
	"testing"
)

// statusLine builds one line of `rex events --json` for a status change.
func statusLine(block, id, state string) string {
	return fmt.Sprintf(`{"type":"session_event","payload":{"event":{"type":"block_event","payload":`+
		`{"block_id":%q,"name":"program_status_changed","payload":{"id":%q,"record":{"state":%q}}}}}}`,
		block, id, state)
}

func TestWatchTurnsReportsEachStartOfWork(t *testing.T) {
	events := strings.Join([]string{
		statusLine("block:agent", "", "working"),
		statusLine("block:agent", "", "working"), // a new progress message in the same turn
		statusLine("block:other", "", "idle"),
		statusLine("block:agent", "sub", "done"), // a sub-agent of the same turn
		statusLine("block:agent", "", "done"),
		statusLine("block:other", "", "working"),
		`not json`,
		statusLine("block:agent", "", "working"),
	}, "\n")

	turns := make(chan struct{}, 10)
	watchTurns(strings.NewReader(events), "block:agent", false, turns)

	if got := len(turns); got != 2 {
		t.Errorf("got %d turns, want 2", got)
	}
}

func TestWatchTurnsSkipsTheTurnThatAlreadyRuns(t *testing.T) {
	events := strings.Join([]string{
		statusLine("block:agent", "", "working"),
		statusLine("block:agent", "", "idle"),
	}, "\n")

	turns := make(chan struct{}, 10)
	watchTurns(strings.NewReader(events), "block:agent", true, turns)

	if got := len(turns); got != 0 {
		t.Errorf("got %d turns, want 0", got)
	}
}
