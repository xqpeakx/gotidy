package main

import (
	"path/filepath"
	"sort"
	"strings"
)

const OtherCategory = "other"

type CategoryDefinition struct {
	Name        string   `json:"name"`
	Destination string   `json:"destination"`
	Extensions  []string `json:"extensions"`
}

type CategoryRule struct {
	Name        string `json:"name"`
	Destination string `json:"destination"`
	Origin      string `json:"origin,omitempty"`
}

type CategoryResolver struct {
	byExtension map[string]CategoryRule
}

var defaultCategoryByExtension = map[string]string{
	// Images
	"avif": "images",
	"bmp":  "images",
	"dng":  "images",
	"gif":  "images",
	"heic": "images",
	"heif": "images",
	"ico":  "images",
	"jfif": "images",
	"jpeg": "images",
	"jpg":  "images",
	"png":  "images",
	"raw":  "images",
	"svg":  "images",
	"tif":  "images",
	"tiff": "images",
	"webp": "images",

	// Documents
	"azw":   "documents",
	"azw3":  "documents",
	"doc":   "documents",
	"docm":  "documents",
	"docx":  "documents",
	"epub":  "documents",
	"md":    "documents",
	"mobi":  "documents",
	"odt":   "documents",
	"pages": "documents",
	"pdf":   "documents",
	"rtf":   "documents",
	"tex":   "documents",
	"txt":   "documents",

	// Spreadsheets
	"csv":     "spreadsheets",
	"numbers": "spreadsheets",
	"ods":     "spreadsheets",
	"tsv":     "spreadsheets",
	"xls":     "spreadsheets",
	"xlsm":    "spreadsheets",
	"xlsx":    "spreadsheets",

	// Presentations
	"key":  "presentations",
	"odp":  "presentations",
	"pot":  "presentations",
	"potx": "presentations",
	"pps":  "presentations",
	"ppsx": "presentations",
	"ppt":  "presentations",
	"pptx": "presentations",

	// Video
	"3gp":  "videos",
	"avi":  "videos",
	"flv":  "videos",
	"m2ts": "videos",
	"m4v":  "videos",
	"mkv":  "videos",
	"mov":  "videos",
	"mp4":  "videos",
	"mpeg": "videos",
	"mpg":  "videos",
	"mts":  "videos",
	"webm": "videos",
	"wmv":  "videos",

	// Audio
	"aac":  "audio",
	"aif":  "audio",
	"aiff": "audio",
	"flac": "audio",
	"m4a":  "audio",
	"mid":  "audio",
	"midi": "audio",
	"mp3":  "audio",
	"ogg":  "audio",
	"opus": "audio",
	"wav":  "audio",

	// Archives
	"7z":  "archives",
	"apk": "archives",
	"bz2": "archives",
	"cab": "archives",
	"deb": "archives",
	"dmg": "archives",
	"gz":  "archives",
	"iso": "archives",
	"pkg": "archives",
	"rar": "archives",
	"rpm": "archives",
	"tar": "archives",
	"tgz": "archives",
	"xz":  "archives",
	"zst": "archives",
	"zip": "archives",

	// Code
	"asm":    "code",
	"bat":    "code",
	"c":      "code",
	"cfg":    "code",
	"conf":   "code",
	"cpp":    "code",
	"cs":     "code",
	"css":    "code",
	"dart":   "code",
	"env":    "code",
	"gradle": "code",
	"go":     "code",
	"h":      "code",
	"hpp":    "code",
	"html":   "code",
	"ini":    "code",
	"ipynb":  "code",
	"java":   "code",
	"js":     "code",
	"json":   "code",
	"jsx":    "code",
	"kt":     "code",
	"kts":    "code",
	"lock":   "code",
	"lua":    "code",
	"pl":     "code",
	"php":    "code",
	"proto":  "code",
	"ps1":    "code",
	"py":     "code",
	"rb":     "code",
	"rs":     "code",
	"scala":  "code",
	"sh":     "code",
	"sql":    "code",
	"svelte": "code",
	"swift":  "code",
	"toml":   "code",
	"ts":     "code",
	"tsx":    "code",
	"vue":    "code",
	"wasm":   "code",
	"xml":    "code",
	"yaml":   "code",
	"yml":    "code",
}

var defaultCategoryResolver = mustDefaultCategoryResolver()

func mustDefaultCategoryResolver() CategoryResolver {
	resolver, err := NewCategoryResolver(nil)
	if err != nil {
		panic(err)
	}
	return resolver
}

func NewCategoryResolver(config *Config) (CategoryResolver, error) {
	byExtension := make(map[string]CategoryRule, len(defaultCategoryByExtension))
	for ext, category := range defaultCategoryByExtension {
		byExtension[ext] = CategoryRule{Name: category, Destination: category, Origin: "builtin"}
	}

	if config == nil {
		return CategoryResolver{byExtension: byExtension}, nil
	}

	for categoryName, category := range config.Categories {
		name := strings.TrimSpace(categoryName)
		if name == "" {
			return CategoryResolver{}, errInvalidConfig("category name cannot be empty")
		}

		destination := strings.TrimSpace(category.Destination)
		if destination == "" {
			destination = name
		}

		for _, ext := range category.Extensions {
			normalized := normalizeExtension(ext)
			if normalized == "" {
				return CategoryResolver{}, errInvalidConfig("category %q contains an empty extension", name)
			}
			byExtension[normalized] = CategoryRule{Name: name, Destination: destination, Origin: "config"}
		}
	}

	return CategoryResolver{byExtension: byExtension}, nil
}

func DefaultCategoryResolver() CategoryResolver {
	return defaultCategoryResolver
}

func CategoryFor(filename string) string {
	return defaultCategoryResolver.Resolve(filename).Name
}

func (r CategoryResolver) Resolve(filename string) CategoryRule {
	base := filepath.Base(filename)
	if strings.HasPrefix(base, ".") {
		return CategoryRule{Name: OtherCategory, Destination: OtherCategory, Origin: "builtin"}
	}

	ext := normalizeExtension(filepath.Ext(base))
	if ext == "" {
		return CategoryRule{Name: OtherCategory, Destination: OtherCategory, Origin: "builtin"}
	}
	if category, ok := r.byExtension[ext]; ok {
		return category
	}
	return CategoryRule{Name: OtherCategory, Destination: OtherCategory, Origin: "builtin"}
}

func CategoryDefinitionsList() []CategoryDefinition {
	return defaultCategoryResolver.Definitions()
}

func (r CategoryResolver) Definitions() []CategoryDefinition {
	type ruleKey struct {
		Name        string
		Destination string
	}

	byRule := make(map[ruleKey][]string)
	for ext, rule := range r.byExtension {
		key := ruleKey{Name: rule.Name, Destination: rule.Destination}
		byRule[key] = append(byRule[key], ext)
	}
	byRule[ruleKey{Name: OtherCategory, Destination: OtherCategory}] = nil

	keys := make([]ruleKey, 0, len(byRule))
	for key := range byRule {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Name != keys[j].Name {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].Destination < keys[j].Destination
	})

	definitions := make([]CategoryDefinition, 0, len(keys))
	for _, key := range keys {
		extensions := append([]string(nil), byRule[key]...)
		sort.Strings(extensions)
		definitions = append(definitions, CategoryDefinition{
			Name:        key.Name,
			Destination: key.Destination,
			Extensions:  extensions,
		})
	}

	return definitions
}

func normalizeExtension(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}
