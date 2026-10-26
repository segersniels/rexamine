package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveThenLoadKeepsTheSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if err := Save(Config{HideTests: true}); err != nil {
		t.Fatal(err)
	}
	if got := Load(); !got.HideTests {
		t.Errorf("Load() = %+v, want HideTests", got)
	}
}

func TestLoadWithoutAFileGivesTheDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if got := Load(); got != (Config{}) {
		t.Errorf("Load() = %+v, want the defaults", got)
	}
}

func TestLoadWithABrokenFileGivesTheDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "rexdiff"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rexdiff", "config.json"), []byte(`{"hideTests": tru`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := Load(); got != (Config{}) {
		t.Errorf("Load() = %+v, want the defaults", got)
	}
}
