package agent

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// statusLine builds one line of `rex events --json` for a status change.
func statusLine(block, id, state string) string {
	return fmt.Sprintf(`{"type":"session_event","payload":{"event":{"type":"block_event","payload":`+
		`{"block_id":%q,"name":"program_status_changed","payload":{"id":%q,"record":{"state":%q}}}}}}`,
		block, id, state)
}

func TestWatchTurnsReportsEachStartAndEndOfWork(t *testing.T) {
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

	turns := make(chan TurnEvent, 10)
	watchTurns(strings.NewReader(events), "block:agent", false, turns)
	close(turns)

	var got []TurnEvent
	for event := range turns {
		got = append(got, event)
	}
	if want := []TurnEvent{TurnStarted, TurnEnded, TurnStarted}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWatchTurnsEndsTheTurnThatAlreadyRuns(t *testing.T) {
	events := strings.Join([]string{
		statusLine("block:agent", "", "working"),
		statusLine("block:agent", "", "idle"),
	}, "\n")

	turns := make(chan TurnEvent, 10)
	watchTurns(strings.NewReader(events), "block:agent", true, turns)
	close(turns)

	var got []TurnEvent
	for event := range turns {
		got = append(got, event)
	}
	if want := []TurnEvent{TurnEnded}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
