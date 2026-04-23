package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// writeFiles creates every named file inside dir with empty contents. It fails
// the test immediately if any file cannot be created.
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatalf("writeFiles %q: %v", path, err)
		}
	}
}

func writeSizedFile(t *testing.T, dir, name string, size int) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatalf("writeSizedFile %q: %v", path, err)
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
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func TestOrganize_MovesFilesIntoCategories(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md", "song.mp3", "script.go", "unknown.xyz")

	summary, err := Organize(dir, Options{})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 5 {
		t.Fatalf("Moved = %d, want 5", summary.Moved)
	}
	if summary.TotalBytes != 0 {
		t.Fatalf("TotalBytes = %d, want 0 for empty fixtures", summary.TotalBytes)
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
			t.Fatalf("expected %q to exist after organize: %v", rel, err)
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
		t.Fatalf("Moved = %d, want 2", summary.Moved)
	}

	after := listDir(t, dir)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("directory changed during dry run: before=%v after=%v", before, after)
	}
}

func TestOrganize_UsesCustomConfigAndDateBuckets(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	path := filepath.Join(dir, "photo.jpg")
	modTime := time.Date(2024, time.March, 9, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	resolver, err := NewCategoryResolver(&Config{
		Categories: map[string]ConfiguredCategory{
			"photos": {
				Extensions:  []string{"jpg"},
				Destination: "Projects/Photography",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewCategoryResolver: %v", err)
	}

	summary, err := Organize(dir, Options{
		ByDate:   true,
		Resolver: resolver,
	})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.ByCategory["photos"] != 1 {
		t.Fatalf("ByCategory[photos] = %d, want 1", summary.ByCategory["photos"])
	}
	if _, err := os.Stat(filepath.Join(dir, "Projects", "Photography", "2024", "03", "photo.jpg")); err != nil {
		t.Fatalf("photo should be moved into custom dated destination: %v", err)
	}
}

func TestOrganize_FiltersByIncludeExcludeAndIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "keep.pdf", "skip.tmp", "ignore.docx")

	summary, err := Organize(dir, Options{
		IncludePatterns: []string{"*.pdf", "*.docx"},
		ExcludePatterns: []string{"*.tmp"},
		IgnorePatterns:  []string{"ignore.*"},
	})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 1 {
		t.Fatalf("Moved = %d, want 1", summary.Moved)
	}
	if summary.Filtered != 2 {
		t.Fatalf("Filtered = %d, want 2", summary.Filtered)
	}
	if _, err := os.Stat(filepath.Join(dir, "documents", "keep.pdf")); err != nil {
		t.Fatalf("keep.pdf should move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skip.tmp")); err != nil {
		t.Fatalf("skip.tmp should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ignore.docx")); err != nil {
		t.Fatalf("ignore.docx should remain: %v", err)
	}
}

func TestOrganize_RenamesDestinationCollisionWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFiles(t, filepath.Join(dir, "images"), "photo.jpg")
	writeFiles(t, dir, "photo.jpg")

	summary, err := Organize(dir, Options{CollisionStrategy: DuplicateRename})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 1 {
		t.Fatalf("Moved = %d, want 1", summary.Moved)
	}
	if summary.Renamed != 1 {
		t.Fatalf("Renamed = %d, want 1", summary.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "images", "photo_1.jpg")); err != nil {
		t.Fatalf("renamed destination should exist: %v", err)
	}
}

func TestOrganize_BySizePrefixesLargeFiles(t *testing.T) {
	dir := t.TempDir()
	writeSizedFile(t, dir, "video.mp4", 2048)

	summary, err := Organize(dir, Options{
		BySize:             true,
		LargeFileThreshold: 1024,
	})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.TotalBytes != 2048 {
		t.Fatalf("TotalBytes = %d, want 2048", summary.TotalBytes)
	}
	if _, err := os.Stat(filepath.Join(dir, "large_files", "videos", "video.mp4")); err != nil {
		t.Fatalf("large file should move under large_files/videos: %v", err)
	}
}

func TestOrganize_BackupCreatesZip(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	summary, err := Organize(dir, Options{Backup: true})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.BackupPath == "" {
		t.Fatal("BackupPath = empty, want a created backup")
	}
	if _, err := os.Stat(summary.BackupPath); err != nil {
		t.Fatalf("backup file should exist: %v", err)
	}
}

func TestOrganize_InteractiveCanSkipFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	summary, err := Organize(dir, Options{
		Interactive: true,
		Prompt: func(prompt string) (string, error) {
			return "n", nil
		},
	})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}

	if summary.Moved != 0 {
		t.Fatalf("Moved = %d, want 0", summary.Moved)
	}
	if summary.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", summary.Skipped)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should remain in place: %v", err)
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
		t.Fatalf("WriteFile: %v", err)
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
	summary, err := Organize(dir, Options{
		Verbose: true,
		Logf: func(format string, args ...any) {
			messages = append(messages, fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		t.Fatalf("Organize: %v", err)
	}
	if summary.Moved != 1 {
		t.Fatalf("Moved = %d, want 1", summary.Moved)
	}
	if len(messages) == 0 {
		t.Fatal("expected at least one verbose log message")
	}
	if got := messages[0]; got != "moved photo.jpg to images/photo.jpg" {
		t.Fatalf("first verbose log = %q, want %q", got, "moved photo.jpg to images/photo.jpg")
	}
}
