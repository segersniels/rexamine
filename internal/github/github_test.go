package github

import "testing"

func TestRepository(t *testing.T) {
	tests := map[string]string{
		"https://github.com/tallyforms/tally/pull/4815": "tallyforms/tally",
		"https://ghe.example.com/team/app/pull/1":       "team/app",
		"not a url": "",
	}

	for url, want := range tests {
		if got := Repository(url); got != want {
			t.Errorf("Repository(%q) = %q, want %q", url, got, want)
		}
	}
}
