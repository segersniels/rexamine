package agent

import (
	"testing"

	"github.com/segersniels/rexamine/internal/diff"
)

func TestLocation(t *testing.T) {
	tests := []struct {
		name  string
		lines []diff.Line
		want  string
	}{
		{"one line", []diff.Line{{Kind: diff.Added, NewLine: 7}}, "L7"},
		{
			"range skips removed lines",
			[]diff.Line{
				{Kind: diff.Context, OldLine: 2, NewLine: 2},
				{Kind: diff.Removed, OldLine: 3},
				{Kind: diff.Added, NewLine: 3},
				{Kind: diff.Context, OldLine: 4, NewLine: 4},
			},
			"L2-L4",
		},
		{
			"only removed lines",
			[]diff.Line{{Kind: diff.Removed, OldLine: 10}, {Kind: diff.Removed, OldLine: 11}},
			"L10-L11 (removed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Snippet{Lines: tt.lines}).Location(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	snippet := Snippet{
		Path: "src/settings-mapper.ts",
		Lines: []diff.Line{
			{Kind: diff.Context, Text: "  type Language,", OldLine: 2, NewLine: 2},
			{Kind: diff.Added, Text: "  truncate,", NewLine: 3},
		},
	}

	got := Format(snippet, "  Why do we still import truncate?  ")
	want := "src/settings-mapper.ts L2-L3:\n```diff\n   type Language,\n+  truncate,\n```\nWhy do we still import truncate?\n\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatLengthensFenceAroundBackticks(t *testing.T) {
	snippet := Snippet{
		Path:  "README.md",
		Lines: []diff.Line{{Kind: diff.Added, Text: "```sh", NewLine: 1}},
	}

	want := "README.md L1:\n````diff\n+```sh\n````\n\n"
	if got := Format(snippet, ""); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
