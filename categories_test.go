package main

import "testing"

func TestCategoryFor(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{"simple jpg", "photo.jpg", "images"},
		{"uppercase extension", "PHOTO.JPG", "images"},
		{"mixed case extension", "Report.PdF", "documents"},
		{"heic image", "holiday.heic", "images"},
		{"markdown", "README.md", "documents"},
		{"ebook", "book.epub", "documents"},
		{"go source", "main.go", "code"},
		{"swift source", "App.SWIFT", "code"},
		{"yaml config", "config.yml", "code"},
		{"disk image", "installer.dmg", "archives"},
		{"archive", "backup.tar.gz", "archives"},
		{"multiple dots", "my.weird.name.mp3", "audio"},
		{"no extension", "Makefile", OtherCategory},
		{"unknown extension", "file.xyz", OtherCategory},
		{"empty string", "", OtherCategory},
		{"dotfile with no ext", ".env", OtherCategory},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CategoryFor(tc.filename)
			if got != tc.want {
				t.Errorf("CategoryFor(%q) = %q, want %q", tc.filename, got, tc.want)
			}
		})
	}
}

func TestCategoryDefinitionsList(t *testing.T) {
	definitions := CategoryDefinitionsList()
	if len(definitions) == 0 {
		t.Fatal("CategoryDefinitionsList returned no definitions")
	}

	seenOther := false
	for i, definition := range definitions {
		if i > 0 && definitions[i-1].Name > definition.Name {
			t.Fatalf("definitions are not sorted: %q before %q", definitions[i-1].Name, definition.Name)
		}
		if definition.Name == OtherCategory {
			seenOther = true
			if definition.Destination != OtherCategory {
				t.Fatalf("other category destination = %q, want %q", definition.Destination, OtherCategory)
			}
			if len(definition.Extensions) != 0 {
				t.Fatalf("other category should not have mapped extensions, got %v", definition.Extensions)
			}
		}
	}
	if !seenOther {
		t.Fatalf("definitions did not include %q", OtherCategory)
	}
}

func TestNewCategoryResolver_WithCustomDestination(t *testing.T) {
	resolver, err := NewCategoryResolver(&Config{
		Categories: map[string]ConfiguredCategory{
			"photos": {
				Extensions:  []string{"jpg", "raw"},
				Destination: "Projects/Photography",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewCategoryResolver: %v", err)
	}

	rule := resolver.Resolve("photo.jpg")
	if rule.Name != "photos" {
		t.Fatalf("rule.Name = %q, want %q", rule.Name, "photos")
	}
	if rule.Destination != "Projects/Photography" {
		t.Fatalf("rule.Destination = %q, want %q", rule.Destination, "Projects/Photography")
	}
}
