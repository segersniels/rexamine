package diff

import (
	"reflect"
	"testing"
)

func TestParseNumbersLines(t *testing.T) {
	files := Parse(`diff --git a/main.go b/main.go
index 728c6ef..16b31e3 100644
--- a/main.go
+++ b/main.go
@@ -10,4 +10,5 @@ func main() {
 	keep()
-	old()
+	newer()
+	added()
 	keep()
\ No newline at end of file
`)

	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}

	want := []Line{
		{Kind: Context, Text: "\tkeep()", OldLine: 10, NewLine: 10},
		{Kind: Removed, Text: "\told()", OldLine: 11},
		{Kind: Added, Text: "\tnewer()", NewLine: 11},
		{Kind: Added, Text: "\tadded()", NewLine: 12},
		{Kind: Context, Text: "\tkeep()", OldLine: 12, NewLine: 13},
	}
	if got := files[0].Hunks[0].Lines; !reflect.DeepEqual(got, want) {
		t.Errorf("lines:\n got %+v\nwant %+v", got, want)
	}

	if added, removed := files[0].Stats(); added != 2 || removed != 1 {
		t.Errorf("stats: got +%d -%d, want +2 -1", added, removed)
	}
}

func TestParseFileStates(t *testing.T) {
	files := Parse(`diff --git a/new.txt b/new.txt
new file mode 100644
--- /dev/null
+++ b/new.txt
@@ -0,0 +1 @@
+hello
diff --git a/gone.txt b/gone.txt
deleted file mode 100644
--- a/gone.txt
+++ /dev/null
@@ -1 +0,0 @@
-bye
diff --git a/old/name.go b/new/name.go
similarity index 100%
rename from old/name.go
rename to new/name.go
diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
`)

	if len(files) != 4 {
		t.Fatalf("got %d files, want 4", len(files))
	}
	if !files[0].IsNew || files[0].Path != "new.txt" {
		t.Errorf("new file: got %+v", files[0])
	}
	if !files[1].IsDeleted || files[1].Path != "gone.txt" {
		t.Errorf("deleted file: got %+v", files[1])
	}
	if !files[2].IsRenamed() || files[2].OldPath != "old/name.go" || files[2].Path != "new/name.go" {
		t.Errorf("renamed file: got %+v", files[2])
	}
	if !files[3].IsBinary || len(files[3].Hunks) != 0 {
		t.Errorf("binary file: got %+v", files[3])
	}
}

func TestParseKeepsDashLinesInsideHunks(t *testing.T) {
	files := Parse(`diff --git a/notes.md b/notes.md
--- a/notes.md
+++ b/notes.md
@@ -1,2 +1,2 @@
--- a removed rule
+++ an added heading
`)

	lines := files[0].Hunks[0].Lines
	if len(lines) != 2 || lines[0].Text != "-- a removed rule" || lines[1].Text != "++ an added heading" {
		t.Errorf("got %+v", lines)
	}
	if files[0].Path != "notes.md" {
		t.Errorf("path changed by a hunk line: %q", files[0].Path)
	}
}
