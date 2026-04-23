package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gotidy.json")
	data := `{
  "categories": {
    "photos": {
      "extensions": ["jpg", "dng"],
      "destination": "Projects/Photography"
    }
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadConfig(dir, "")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if loaded.Path != path {
		t.Fatalf("loaded.Path = %q, want %q", loaded.Path, path)
	}
	if got := loaded.Config.Categories["photos"].Destination; got != "Projects/Photography" {
		t.Fatalf("destination = %q, want %q", got, "Projects/Photography")
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gotidy.yaml")
	data := `categories:
  photos:
    extensions: [jpg, png, raw, dng]
    destination: Projects/Photography
  work_docs:
    extensions:
      - pdf
      - docx
    destination: Work/Documents
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadConfig(dir, "")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if got := loaded.Config.Categories["photos"].Extensions; len(got) != 4 {
		t.Fatalf("extensions = %v, want 4 entries", got)
	}
	if got := loaded.Config.Categories["work_docs"].Destination; got != "Work/Documents" {
		t.Fatalf("destination = %q, want %q", got, "Work/Documents")
	}
}
