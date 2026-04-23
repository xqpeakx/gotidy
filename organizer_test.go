package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeFiles creates every named file inside dir with empty contents. It fails
// the test immediately if any file cannot be created.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		path := filepath.Join(dir, n)
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatalf("writeFiles %q: %v", path, err)
		}
	}
}

// listDir returns a sorted slice of the names in dir.
func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readDir %q: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestOrganize_MovesFilesIntoCategories(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir,
		"photo.jpg",
		"notes.md",
		"song.mp3",
		"script.go",
		"unknown.xyz",
	)

	summary, err := Organize(dir, Options{})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 5 {
		t.Errorf("Moved = %d, want 5", summary.Moved)
	}
	if summary.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", summary.Skipped)
	}

	expectedPaths := []string{
		"images/photo.jpg",
		"documents/notes.md",
		"audio/song.mp3",
		"code/script.go",
		"other/unknown.xyz",
	}
	for _, rel := range expectedPaths {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("expected %q to exist after organize: %v", rel, err)
		}
	}

	expectedCounts := map[string]int{
		"images":    1,
		"documents": 1,
		"audio":     1,
		"code":      1,
		"other":     1,
	}
	for cat, want := range expectedCounts {
		if got := summary.ByCategory[cat]; got != want {
			t.Errorf("ByCategory[%q] = %d, want %d", cat, got, want)
		}
	}
}

func TestOrganize_DryRunDoesNotMove(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md")

	before := listDir(t, dir)

	summary, err := Organize(dir, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 2 {
		t.Errorf("Moved = %d, want 2 (dry run still reports what would move)", summary.Moved)
	}

	after := listDir(t, dir)
	if len(before) != len(after) {
		t.Errorf("directory changed during dry run: before=%v after=%v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("directory changed during dry run: before=%v after=%v", before, after)
			break
		}
	}
}

func TestOrganize_SkipsHiddenFilesAndSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, ".hidden", "photo.jpg")
	if err := os.Mkdir(filepath.Join(dir, "preexisting"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	summary, err := Organize(dir, Options{})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 1 {
		t.Errorf("Moved = %d, want 1 (only photo.jpg should move)", summary.Moved)
	}

	// The hidden file and the pre-existing directory should still be at the top level.
	if _, err := os.Stat(filepath.Join(dir, ".hidden")); err != nil {
		t.Errorf(".hidden should remain at top level: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "preexisting")); err != nil || !info.IsDir() {
		t.Errorf("preexisting subdirectory should remain: err=%v", err)
	}
}

func TestOrganize_SkipsDestinationCollision(t *testing.T) {
	dir := t.TempDir()

	// Pre-create an images/photo.jpg so the Organize call has to skip.
	if err := os.Mkdir(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFiles(t, filepath.Join(dir, "images"), "photo.jpg")
	writeFiles(t, dir, "photo.jpg")

	summary, err := Organize(dir, Options{})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 0 {
		t.Errorf("Moved = %d, want 0 (collision should be skipped)", summary.Moved)
	}
	if summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", summary.Skipped)
	}

	// The original top-level photo.jpg should still be there.
	if _, err := os.Stat(filepath.Join(dir, "photo.jpg")); err != nil {
		t.Errorf("top-level photo.jpg should remain on collision: %v", err)
	}
}

func TestOrganize_NonExistentDirectory(t *testing.T) {
	_, err := Organize(filepath.Join(t.TempDir(), "does-not-exist"), Options{})
	if err == nil {
		t.Fatal("expected error for missing directory, got nil")
	}
}

func TestOrganize_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	_, err := Organize(file, Options{})
	if err == nil {
		t.Fatal("expected error when target is a regular file, got nil")
	}
}

func TestOrganize_VerboseLoggingIsInvoked(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	var messages []string
	opts := Options{
		Verbose: true,
		Logf: func(format string, args ...any) {
			messages = append(messages, format)
		},
	}

	if _, err := Organize(dir, opts); err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if len(messages) == 0 {
		t.Error("expected at least one verbose log message, got none")
	}
}
