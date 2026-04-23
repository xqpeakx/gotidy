package main

import (
	"path/filepath"
	"strings"
)

const OtherCategory = "other"

var categoryByExtension = map[string]string{
	// Images
	"bmp":  "images",
	"gif":  "images",
	"ico":  "images",
	"jpeg": "images",
	"jpg":  "images",
	"png":  "images",
	"svg":  "images",
	"tif":  "images",
	"tiff": "images",
	"webp": "images",

	// Documents
	"doc":  "documents",
	"docx": "documents",
	"md":   "documents",
	"odt":  "documents",
	"pdf":  "documents",
	"rtf":  "documents",
	"txt":  "documents",

	// Spreadsheets
	"csv":  "spreadsheets",
	"ods":  "spreadsheets",
	"tsv":  "spreadsheets",
	"xls":  "spreadsheets",
	"xlsx": "spreadsheets",

	// Presentations
	"key":  "presentations",
	"odp":  "presentations",
	"ppt":  "presentations",
	"pptx": "presentations",

	// Video
	"avi":  "videos",
	"flv":  "videos",
	"mkv":  "videos",
	"mov":  "videos",
	"mp4":  "videos",
	"webm": "videos",
	"wmv":  "videos",

	// Audio
	"aac":  "audio",
	"flac": "audio",
	"m4a":  "audio",
	"mp3":  "audio",
	"ogg":  "audio",
	"wav":  "audio",

	// Archives
	"7z":  "archives",
	"bz2": "archives",
	"gz":  "archives",
	"rar": "archives",
	"tar": "archives",
	"xz":  "archives",
	"zip": "archives",

	// Code
	"c":    "code",
	"cpp":  "code",
	"cs":   "code",
	"css":  "code",
	"go":   "code",
	"h":    "code",
	"hpp":  "code",
	"html": "code",
	"java": "code",
	"js":   "code",
	"json": "code",
	"jsx":  "code",
	"php":  "code",
	"py":   "code",
	"rb":   "code",
	"rs":   "code",
	"sh":   "code",
	"toml": "code",
	"ts":   "code",
	"tsx":  "code",
	"xml":  "code",
	"yaml": "code",
	"yml":  "code",
}

func CategoryFor(filename string) string {
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	if ext == "" {
		return OtherCategory
	}
	if category, ok := categoryByExtension[strings.ToLower(ext)]; ok {
		return category
	}
	return OtherCategory
}
