package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v0.2.0", "0.1.0", true},
		{"v0.10.0", "0.9.9", true},
		{"v1.0.0", "0.99.0", true},
		{"v0.2.0", "0.2.0", false},
		{"v0.1.0", "0.2.0", false},
		{"v0.2.0", "v0.1.0-1-g74132e5", true},
		{"v0.1.0", "v0.1.0-1-g74132e5-dirty", false},
		{"v0.2.0", "dev", false},
		{"", "0.1.0", false},
	}

	for _, tt := range tests {
		if got := newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("newer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

// writeCache saves a cache file that was checked at the given time.
func writeCache(t *testing.T, path string, checkedAt time.Time, latest string) {
	t.Helper()
	content, err := json.Marshal(cache{CheckedAt: checkedAt, Latest: latest})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLatestVersionUsesAFreshCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.json")
	writeCache(t, path, time.Now().Add(-checkInterval/2), "v0.2.0")

	fetch := func() (string, error) {
		t.Error("fetched although the cache is fresh")
		return "", nil
	}
	if got, err := latestVersion(path, fetch); err != nil || got != "v0.2.0" {
		t.Errorf("got %q, %v; want v0.2.0 from the cache", got, err)
	}
}

func TestLatestVersionRefreshesAStaleCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rexamine", "update.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeCache(t, path, time.Now().Add(-2*checkInterval), "v0.1.0")

	fetch := func() (string, error) { return "v0.3.0", nil }
	if got, err := latestVersion(path, fetch); err != nil || got != "v0.3.0" {
		t.Fatalf("got %q, %v; want v0.3.0 from GitHub", got, err)
	}

	// The next run uses the saved answer.
	failing := func() (string, error) { return "", errors.New("offline") }
	if got, err := latestVersion(path, failing); err != nil || got != "v0.3.0" {
		t.Errorf("got %q, %v; want v0.3.0 from the cache", got, err)
	}
}

func TestCommandForMatchesTheInstallation(t *testing.T) {
	script := "curl -fsSL https://raw.githubusercontent.com/segersniels/rexamine/main/scripts/install.sh | "
	tests := []struct {
		name, path, want string
	}{
		{"npm", "/opt/homebrew/lib/node_modules/@segersniels/rexamine-darwin-arm64/bin/rexamine", "npm install -g @segersniels/rexamine && rexamine setup"},
		{"script default", "/Users/me/.local/bin/rexamine", script + "sh"},
		{"script elsewhere", "/usr/local/bin/rexamine", script + "REXAMINE_INSTALL_DIR=/usr/local/bin sh"},
		{"path with a space", "/Users/me/my tools/rexamine", script + "REXAMINE_INSTALL_DIR='/Users/me/my tools' sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commandFor(tt.path, "/Users/me"); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}
