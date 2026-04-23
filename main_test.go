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
	if !strings.Contains(output, "Would move 2 file(s) in "+dir+" (0 skipped).") {
		t.Fatalf("stdout = %q, want dry-run summary", output)
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
