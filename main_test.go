package main

import (
	"bytes"
	"encoding/json"
	"io"
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

func TestRun_VersionJSON(t *testing.T) {
	oldVersion := version
	version = "1.2.3"
	t.Cleanup(func() { version = oldVersion })

	var stdout, stderr bytes.Buffer

	code := run([]string{"--version", "--json"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--version --json) exit code = %d, want 0", code)
	}

	var payload struct {
		Mode    string `json:"mode"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if payload.Mode != "version" || payload.Name != "gotidy" || payload.Version != "1.2.3" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_Update(t *testing.T) {
	oldUpdater := selfUpdate
	t.Cleanup(func() { selfUpdate = oldUpdater })

	called := false
	selfUpdate = func(stdout, stderr io.Writer) error {
		called = true
		_, _ = io.WriteString(stdout, "updated\n")
		return nil
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"--update"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--update) exit code = %d, want 0", code)
	}
	if !called {
		t.Fatal("expected selfUpdate to be called")
	}
	if got, want := stdout.String(), "updated\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_UpdateCannotBeCombined(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--update", "--verbose"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run(--update --verbose) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--update cannot be combined") {
		t.Fatalf("stderr = %q, want update usage error", stderr.String())
	}
}

func TestRun_ListCategories_WithConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".gotidy.json")
	data := `{
  "categories": {
    "photos": {
      "extensions": ["jpg"],
      "destination": "Projects/Photography"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"--list-categories", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--list-categories) exit code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "photos -> Projects/Photography:") {
		t.Fatalf("stdout = %q, want custom destination line", output)
	}
	if !strings.Contains(output, "Loaded config from "+configPath+".") {
		t.Fatalf("stdout = %q, want config path note", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_Classify_WithConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".gotidy.yaml")
	data := `categories:
  photos:
    extensions: [jpg]
    destination: Projects/Photography
`
	if err := os.WriteFile(configPath, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stdout, stderr bytes.Buffer

	code := run([]string{"--classify", "--config", configPath, "photo.jpg", "report.pdf"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--classify) exit code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "photo.jpg: photos -> Projects/Photography") {
		t.Fatalf("stdout = %q, want custom classification", output)
	}
	if !strings.Contains(output, "report.pdf: documents") {
		t.Fatalf("stdout = %q, want built-in classification", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_ClassifyRequiresArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--classify"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run(--classify) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--classify needs at least one filename") {
		t.Fatalf("stderr = %q, want classify error", stderr.String())
	}
}

func TestRun_JSONCannotBeCombinedWithVerbose(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--json", "--verbose"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run(--json --verbose) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--json cannot be combined with --verbose") {
		t.Fatalf("stderr = %q, want json/verbose error", stderr.String())
	}
}

func TestRun_OverwriteRequiresInteractive(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--overwrite"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run(--overwrite) exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--overwrite requires --interactive") {
		t.Fatalf("stderr = %q, want overwrite error", stderr.String())
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
	if !strings.Contains(stdout.String(), "Restored 1 file in "+dir+".") {
		t.Fatalf("stdout = %q, want undo summary", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_JSONSummary(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg", "notes.md")

	var stdout, stderr bytes.Buffer

	code := run([]string{"--dry-run", "--json", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--dry-run --json) exit code = %d, want 0", code)
	}

	var payload struct {
		Mode            string           `json:"mode"`
		Directory       string           `json:"directory"`
		DryRun          bool             `json:"dry_run"`
		Moved           int              `json:"moved"`
		Renamed         int              `json:"renamed"`
		Overwritten     int              `json:"overwritten"`
		Skipped         int              `json:"skipped"`
		Filtered        int              `json:"filtered"`
		TotalSizeBytes  int64            `json:"total_size_bytes"`
		ByCategory      map[string]int   `json:"by_category"`
		ByCategoryBytes map[string]int64 `json:"by_category_bytes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid json: %v\n%s", err, stdout.String())
	}
	if payload.Mode != "organize" || payload.Directory != dir || !payload.DryRun {
		t.Fatalf("unexpected payload header: %#v", payload)
	}
	if payload.Moved != 2 || payload.Renamed != 0 || payload.Overwritten != 0 || payload.Skipped != 0 {
		t.Fatalf("unexpected payload counts: %#v", payload)
	}
	if payload.ByCategory["documents"] != 1 || payload.ByCategory["images"] != 1 {
		t.Fatalf("unexpected by_category: %#v", payload.ByCategory)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_StatsAndBackup(t *testing.T) {
	dir := t.TempDir()
	writeSizedFile(t, dir, "video.mp4", 2048)

	var stdout, stderr bytes.Buffer

	code := run([]string{"--backup", "--stats", "--by-size", "--large-files-over", "1KB", dir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--backup --stats) exit code = %d, want 0", code)
	}
	output := stdout.String()
	if !strings.Contains(output, "Created backup at ") {
		t.Fatalf("stdout = %q, want backup line", output)
	}
	if !strings.Contains(output, "Total size organized: 2.00 KB.") {
		t.Fatalf("stdout = %q, want size stats", output)
	}
	if !strings.Contains(output, "Filtered") && !strings.Contains(output, "Examined 1 entry.") {
		t.Fatalf("stdout = %q, want stats header", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRun_Interactive(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "photo.jpg")

	var stdout, stderr bytes.Buffer

	code := runWithIO([]string{"--interactive", dir}, strings.NewReader("y\n"), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("runWithIO(--interactive) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "move photo.jpg -> images/photo.jpg?") {
		t.Fatalf("stdout = %q, want prompt", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "images", "photo.jpg")); err != nil {
		t.Fatalf("photo.jpg should be moved after interactive approval: %v", err)
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
	if !strings.Contains(stderr.String(), "accepts at most one directory argument") {
		t.Fatalf("stderr = %q, want argument error", stderr.String())
	}
}

func TestRun_NonExistentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")

	var stdout, stderr bytes.Buffer

	code := run([]string{dir}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run(missing dir) exit code = %d, want 1", code)
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
	if got, want := stdout.String(), "Nothing to do in "+dir+".\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
