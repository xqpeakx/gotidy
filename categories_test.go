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
		{"markdown", "README.md", "documents"},
		{"go source", "main.go", "code"},
		{"yaml config", "config.yml", "code"},
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
