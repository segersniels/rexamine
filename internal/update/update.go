// Package update finds out whether a newer release of rexamine exists, and
// how the user can install it.
package update

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// latestReleaseURL is the GitHub API endpoint for the newest release.
const latestReleaseURL = "https://api.github.com/repos/segersniels/rexamine/releases/latest"

// checkInterval is how long a check stays valid. GitHub allows 60 requests an
// hour without a token, so one check an hour stays far below the limit and
// still shows a new release soon.
const checkInterval = time.Hour

// requestTimeout keeps a slow network from holding a check open.
const requestTimeout = 5 * time.Second

// Notice tells the user about a newer release.
type Notice struct {
	Version string
}

// cache is what the check saves between runs.
type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// Check finds out whether a release newer than current exists. It asks
// GitHub at most once an hour and keeps the answer in the user's cache
// directory. A development build or a failed check reports no notice.
func Check(current string) (Notice, bool) {
	if _, ok := parse(current); !ok {
		return Notice{}, false
	}

	dir, err := cacheDir()
	if err != nil {
		return Notice{}, false
	}
	latest, err := latestVersion(filepath.Join(dir, "update.json"), fetchLatest)
	if err != nil || !newer(latest, current) {
		return Notice{}, false
	}
	return Notice{Version: strings.TrimPrefix(latest, "v")}, true
}

// cacheDir is the directory where the check saves its answer.
func cacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rexamine"), nil
}

// RemoveCache removes the saved answer of the check, for an uninstall.
func RemoveCache() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// latestVersion reads the latest version from the cache file, or fetches it
// and saves it when the cache is missing or older than checkInterval.
func latestVersion(path string, fetch func() (string, error)) (string, error) {
	var saved cache
	if content, err := os.ReadFile(path); err == nil && json.Unmarshal(content, &saved) == nil {
		if time.Since(saved.CheckedAt) < checkInterval {
			return saved.Latest, nil
		}
	}

	latest, err := fetch()
	if err != nil {
		return "", err
	}

	// A failed save only means that the next run asks GitHub again.
	content, _ := json.Marshal(cache{CheckedAt: time.Now(), Latest: latest})
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, content, 0o644)
	}
	return latest, nil
}

// fetchLatest asks GitHub for the tag of the newest release, such as "v0.2.0".
func fetchLatest() (string, error) {
	client := http.Client{Timeout: requestTimeout}
	response, err := client.Get(latestReleaseURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", errors.New("GitHub answered " + response.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// newer checks whether version latest comes after version current.
func newer(latest, current string) bool {
	l, okLatest := parse(latest)
	c, okCurrent := parse(current)
	if !okLatest || !okCurrent {
		return false
	}

	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parse reads the major, minor, and patch numbers of a version such as
// "0.2.0", "v0.2.0", or "v0.1.0-1-g74132e5" from git describe.
func parse(version string) ([3]int, bool) {
	var numbers [3]int
	core, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return numbers, false
	}

	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return numbers, false
		}
		numbers[i] = n
	}
	return numbers, true
}

// Command gives the shell command that updates this installation to the
// latest release.
func Command() string {
	binary, err := Binary()
	if err != nil {
		return scriptCommand("")
	}

	home, _ := os.UserHomeDir()
	return commandFor(binary, home)
}

// Binary finds the file of this rexamine binary, after symbolic links.
func Binary() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(binary)
}

// InstalledWithNpm checks whether the binary at path comes from the npm
// package. npm runs the binary from its package in node_modules.
func InstalledWithNpm(path string) bool {
	separator := string(filepath.Separator)
	return strings.Contains(path, separator+"node_modules"+separator)
}

// commandFor picks the update command for the binary at path. Installations
// that do not come from npm come from the install script, which also updates
// the Rex module and the agent skill.
func commandFor(path, home string) string {
	if InstalledWithNpm(path) {
		return "npm install -g @segersniels/rexamine && rexamine setup"
	}

	// The script installs to ~/.local/bin by default. Elsewhere, it must replace the binary in its own directory.
	dir := filepath.Dir(path)
	if dir == filepath.Join(home, ".local", "bin") {
		return scriptCommand("")
	}
	return scriptCommand(dir)
}

// scriptCommand gives the one-line install script, with the install
// directory when it is not the default.
func scriptCommand(dir string) string {
	command := "curl -fsSL https://raw.githubusercontent.com/segersniels/rexamine/main/scripts/install.sh | "
	if dir == "" {
		return command + "sh"
	}

	// Single quotes keep the shell from reading spaces or other special characters in the path.
	if strings.ContainsAny(dir, " '\"$`\\&;|<>()*?[]#~") {
		dir = "'" + strings.ReplaceAll(dir, "'", `'\''`) + "'"
	}
	return command + "REXAMINE_INSTALL_DIR=" + dir + " sh"
}
