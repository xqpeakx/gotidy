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

func TestLoadConfig_MergesProfilesFile(t *testing.T) {
	dir := t.TempDir()

	basePath := filepath.Join(dir, ".gotidy.yaml")
	baseData := `categories:
  photos:
    extensions: [jpg]
    destination: Photos
`
	if err := os.WriteFile(basePath, []byte(baseData), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	profilesPath := filepath.Join(dir, ".gotidy.profiles.yaml")
	profilesData := `profiles:
  work:
    backup: true
    by_date: true
    duplicate_strategy: rename
    include: [*.pdf, *.docx]
    categories:
      reports:
        extensions: [pdf]
        destination: Work/Reports
`
	if err := os.WriteFile(profilesPath, []byte(profilesData), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := LoadConfig(dir, "")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if len(loaded.Paths) != 2 {
		t.Fatalf("len(loaded.Paths) = %d, want 2", len(loaded.Paths))
	}
	if got := loaded.Config.Categories["photos"].Destination; got != "Photos" {
		t.Fatalf("base category destination = %q, want %q", got, "Photos")
	}
	profile, ok := loaded.Config.Profiles["work"]
	if !ok {
		t.Fatal("expected work profile to be loaded")
	}
	if profile.Backup == nil || !*profile.Backup {
		t.Fatalf("profile.Backup = %#v, want true", profile.Backup)
	}
	if profile.DuplicateStrategy != "rename" {
		t.Fatalf("profile.DuplicateStrategy = %q, want %q", profile.DuplicateStrategy, "rename")
	}
	if got := profile.Categories["reports"].Destination; got != "Work/Reports" {
		t.Fatalf("profile category destination = %q, want %q", got, "Work/Reports")
	}
}

func TestConfigEffective_ProfileMergesCategories(t *testing.T) {
	config := Config{
		Categories: map[string]ConfiguredCategory{
			"photos": {
				Extensions:  []string{"jpg"},
				Destination: "Photos",
			},
		},
		Profiles: map[string]ConfiguredProfile{
			"work": {
				Categories: map[string]ConfiguredCategory{
					"reports": {
						Extensions:  []string{"pdf"},
						Destination: "Work/Reports",
					},
				},
			},
		},
	}

	effective, loadedProfile, err := config.Effective("work")
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if loadedProfile == nil || loadedProfile.Name != "work" {
		t.Fatalf("loadedProfile = %#v, want work", loadedProfile)
	}
	if got := effective.Categories["photos"].Destination; got != "Photos" {
		t.Fatalf("photos destination = %q, want %q", got, "Photos")
	}
	if got := effective.Categories["reports"].Destination; got != "Work/Reports" {
		t.Fatalf("reports destination = %q, want %q", got, "Work/Reports")
	}
}
