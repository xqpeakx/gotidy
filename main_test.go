package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--help"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--help) exit code = %d, want 0", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage text", stderr.String())
	}
}

func TestRun_Version(t *testing.T) {
	oldVersion := version
	version = "1.2.3"
	t.Cleanup(func() { version = oldVersion })

	var stdout, stderr bytes.Buffer

	code := run([]string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--version) exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "gotidy 1.2.3\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestResolvedVersion_UsesInjectedVersion(t *testing.T) {
	oldVersion := version
	version = "1.2.3"
	t.Cleanup(func() { version = oldVersion })

	if got, want := resolvedVersion(), "1.2.3"; got != want {
		t.Fatalf("resolvedVersion() = %q, want %q", got, want)
	}
}

func TestRun_Undo(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	if _, err := Organize(dir, Options{}); err != nil {
		t.Fatalf("Organize: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"--undo", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--undo) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Restored 1 file in "+dir+".") {
		t.Fatalf("stdout = %q, want undo summary", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should be restored after undo: %v", err)
	}
}

func TestRun_TooManyArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"first", "second"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run(too many args) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "accepts at most one directory argument") {
		t.Fatalf("stderr = %q, want argument error", stderr.String())
	}
}

func TestRun_DryRunSummary(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md")

	var stdout, stderr bytes.Buffer

	code := run([]string{"--dry-run", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--dry-run) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Dry run: would move 2 files in "+dir+".") {
		t.Fatalf("stdout = %q, want dry-run summary", output)
	}
	if !strings.Contains(output, "By category:") {
		t.Fatalf("stdout = %q, want category heading", output)
	}
	if !strings.Contains(output, "documents:") || !strings.Contains(output, "images:") {
		t.Fatalf("stdout = %q, want category counts", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should remain in place after dry run: %v", err)
	}
}

func TestRun_NonExistentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")

	var stdout, stderr bytes.Buffer

	code := run([]string{dir}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run(missing dir) exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "error: cannot access") {
		t.Fatalf("stderr = %q, want access error", stderr.String())
	}
}

func TestRun_UndoWithoutUndoData(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer

	code := run([]string{"--undo", dir}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run(--undo missing state) exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no undo data found") {
		t.Fatalf("stderr = %q, want missing undo data error", stderr.String())
	}
}

func TestRun_EmptyDirectorySummary(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer

	code := run([]string{dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(empty dir) exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if got, want := stdout.String(), "Nothing to do in "+dir+".\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
