// Package config keeps the settings of the review screen between runs, in
// $XDG_CONFIG_HOME/rexdiff/config.json or ~/.config/rexdiff/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the settings that the review screen changes and keeps.
type Config struct {
	// HideTests leaves the test files out of the diff.
	HideTests bool `json:"hideTests"`
}

// Load reads the settings. A missing or broken file gives the default
// settings, because the settings only change what the screen shows.
func Load() Config {
	path, err := filePath()
	if err != nil {
		return Config{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

// Save writes the settings, and creates their folder when it is missing.
func Save(c Config) error {
	path, err := filePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Remove removes the settings and their folder, for an uninstall.
func Remove() error {
	path, err := filePath()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Dir(path))
}

// filePath finds the settings file in $XDG_CONFIG_HOME, or in ~/.config
// without it.
func filePath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "rexdiff", "config.json"), nil
}
