package source

import (
	"maps"
	"testing"
)

func TestLocalFilesReadTheOldAndTheNewVersion(t *testing.T) {
	dir := newRepo(t)
	review, err := Local{Scope: Uncommitted, Dir: dir}.Load()
	if err != nil {
		t.Fatal(err)
	}

	oldTexts, newTexts := review.Files.ReadFiles(
		[]string{"base.txt", "untracked.txt", "missing file.txt"},
		[]string{"base.txt", "untracked.txt"},
	)
	if want := map[string]string{"base.txt": "base\n"}; !maps.Equal(oldTexts, want) {
		t.Errorf("old texts are %q, want %q", oldTexts, want)
	}
	if want := map[string]string{"base.txt": "base\nedited\n", "untracked.txt": "new\n"}; !maps.Equal(newTexts, want) {
		t.Errorf("new texts are %q, want %q", newTexts, want)
	}
}
