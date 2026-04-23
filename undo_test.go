package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrganize_WritesUndoManifest(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md")

	if _, err := Organize(dir, Options{}); err != nil {
		t.Fatalf("Organize: %v", err)
	}

	manifest, err := readUndoManifest(dir)
	if err != nil {
		t.Fatalf("readUndoManifest: %v", err)
	}
	if len(manifest.Moves) != 2 {
		t.Fatalf("len(manifest.Moves) = %d, want 2", len(manifest.Moves))
	}
	if got, want := manifest.Moves[0], (moveRecord{From: "notes.md", To: "documents/notes.md"}); got != want {
		t.Fatalf("manifest.Moves[0] = %#v, want %#v", got, want)
	}
	if got, want := manifest.Moves[1], (moveRecord{From: "photo.jpg", To: "images/photo.jpg"}); got != want {
		t.Fatalf("manifest.Moves[1] = %#v, want %#v", got, want)
	}
}

func TestUndo_RestoresLastRun(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md")

	if _, err := Organize(dir, Options{}); err != nil {
		t.Fatalf("Organize: %v", err)
	}

	summary, err := Undo(dir, Options{})
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if summary.Moved != 2 {
		t.Fatalf("Moved = %d, want 2", summary.Moved)
	}
	if summary.Skipped != 0 {
		t.Fatalf("Skipped = %d, want 0", summary.Skipped)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should be restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Fatalf("notes.md should be restored: %v", err)
	}
	if _, err := os.Stat(manifestPath(dir)); !os.IsNotExist(err) {
		t.Fatalf("undo manifest should be removed, stat err = %v", err)
	}
}

func TestUndo_DryRunDoesNotMoveFiles(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	if _, err := Organize(dir, Options{}); err != nil {
		t.Fatalf("Organize: %v", err)
	}

	summary, err := Undo(dir, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Undo dry run: %v", err)
	}
	if summary.Moved != 1 {
		t.Fatalf("Moved = %d, want 1", summary.Moved)
	}
	if _, err := os.Stat(filepath.Join(dir, "images", "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should stay in images after dry run: %v", err)
	}
	if _, err := os.Stat(manifestPath(dir)); err != nil {
		t.Fatalf("undo manifest should remain after dry run: %v", err)
	}
}

func TestUndo_KeepsManifestWhenOriginalLocationIsOccupied(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	if _, err := Organize(dir, Options{}); err != nil {
		t.Fatalf("Organize: %v", err)
	}
	writeFiles(t, dir, "photo.jpg")

	summary, err := Undo(dir, Options{})
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if summary.Moved != 0 {
		t.Fatalf("Moved = %d, want 0", summary.Moved)
	}
	if summary.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", summary.Skipped)
	}
	if _, err := readUndoManifest(dir); err != nil {
		t.Fatalf("undo manifest should remain for a skipped file: %v", err)
	}
}
