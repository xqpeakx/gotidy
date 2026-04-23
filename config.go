package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Categories map[string]ConfiguredCategory `json:"categories"`
	Profiles   map[string]ConfiguredProfile  `json:"profiles"`
}

type ConfiguredCategory struct {
	Extensions  []string `json:"extensions"`
	Destination string   `json:"destination"`
}

type ConfiguredProfile struct {
	Categories        map[string]ConfiguredCategory `json:"categories"`
	Backup            *bool                         `json:"backup,omitempty"`
	ByDate            *bool                         `json:"by_date,omitempty"`
	BySize            *bool                         `json:"by_size,omitempty"`
	LargeFilesOver    string                        `json:"large_files_over,omitempty"`
	Include           []string                      `json:"include,omitempty"`
	Exclude           []string                      `json:"exclude,omitempty"`
	IgnorePatterns    []string                      `json:"ignore_patterns,omitempty"`
	DuplicateStrategy string                        `json:"duplicate_strategy,omitempty"`
}

type LoadedConfig struct {
	Path   string
	Paths  []string
	Config Config
}

type LoadedProfile struct {
	Name    string
	Profile ConfiguredProfile
}

var defaultConfigNames = []string{
	".gotidy.yaml",
	".gotidy.yml",
	".gotidy.json",
	".gotidy.profiles.yaml",
	".gotidy.profiles.yml",
	".gotidy.profiles.json",
}

func LoadConfig(root, explicitPath string) (*LoadedConfig, error) {
	paths := make([]string, 0, len(defaultConfigNames))
	if explicitPath != "" {
		paths = append(paths, explicitPath)
	} else {
		for _, name := range defaultConfigNames {
			paths = append(paths, filepath.Join(root, name))
		}
	}

	loadedPaths := make([]string, 0, len(paths))
	merged := Config{
		Categories: make(map[string]ConfiguredCategory),
		Profiles:   make(map[string]ConfiguredProfile),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if explicitPath == "" && os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cannot read config %q: %w", path, err)
		}

		config, err := parseConfig(path, data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse config %q: %w", path, err)
		}
		loadedPaths = append(loadedPaths, path)
		merged = mergeConfigs(merged, config)
	}

	if len(loadedPaths) == 0 {
		return nil, nil
	}

	return &LoadedConfig{
		Path:   strings.Join(loadedPaths, ", "),
		Paths:  loadedPaths,
		Config: merged,
	}, nil
}

func (c Config) Effective(profileName string) (Config, *LoadedProfile, error) {
	effective := Config{
		Categories: cloneCategories(c.Categories),
		Profiles:   make(map[string]ConfiguredProfile),
	}

	if profileName == "" {
		return effective, nil, nil
	}

	profile, ok := c.Profiles[profileName]
	if !ok {
		return Config{}, nil, errInvalidConfig("profile %q was not found", profileName)
	}

	for name, category := range profile.Categories {
		effective.Categories[name] = category
	}

	return effective, &LoadedProfile{Name: profileName, Profile: profile}, nil
}

func parseConfig(path string, data []byte) (Config, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return parseJSONConfig(data)
	case ".yaml", ".yml":
		return parseYAMLConfig(data)
	default:
		return Config{}, errInvalidConfig("unsupported config format %q", filepath.Ext(path))
	}
}

func parseJSONConfig(data []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if config.Categories == nil {
		config.Categories = make(map[string]ConfiguredCategory)
	}
	if config.Profiles == nil {
		config.Profiles = make(map[string]ConfiguredProfile)
	}
	return config, nil
}

func parseYAMLConfig(data []byte) (Config, error) {
	config := Config{
		Categories: make(map[string]ConfiguredCategory),
		Profiles:   make(map[string]ConfiguredProfile),
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	var (
		lineNumber        int
		currentSection    string
		currentProfile    string
		currentCategory   string
		profileCategories bool
		readingList       string
	)

	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		if strings.ContainsRune(rawLine, '\t') {
			return Config{}, errInvalidConfig("line %d uses tabs; use spaces for indentation", lineNumber)
		}

		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " "))

		switch indent {
		case 0:
			switch trimmed {
			case "categories:":
				currentSection = "categories"
				currentProfile = ""
				currentCategory = ""
				profileCategories = false
				readingList = ""
			case "profiles:":
				currentSection = "profiles"
				currentProfile = ""
				currentCategory = ""
				profileCategories = false
				readingList = ""
			default:
				return Config{}, errInvalidConfig("line %d: expected \"categories:\" or \"profiles:\"", lineNumber)
			}
		case 2:
			switch currentSection {
			case "categories":
				if !strings.HasSuffix(trimmed, ":") {
					return Config{}, errInvalidConfig("line %d: expected a category name", lineNumber)
				}
				currentCategory = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
				if currentCategory == "" {
					return Config{}, errInvalidConfig("line %d: category name cannot be empty", lineNumber)
				}
				config.Categories[currentCategory] = ConfiguredCategory{}
				readingList = ""
			case "profiles":
				if !strings.HasSuffix(trimmed, ":") {
					return Config{}, errInvalidConfig("line %d: expected a profile name", lineNumber)
				}
				currentProfile = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
				if currentProfile == "" {
					return Config{}, errInvalidConfig("line %d: profile name cannot be empty", lineNumber)
				}
				if _, ok := config.Profiles[currentProfile]; !ok {
					config.Profiles[currentProfile] = ConfiguredProfile{Categories: make(map[string]ConfiguredCategory)}
				}
				currentCategory = ""
				profileCategories = false
				readingList = ""
			default:
				return Config{}, errInvalidConfig("line %d: section missing before nested key", lineNumber)
			}
		case 4:
			switch currentSection {
			case "categories":
				if currentCategory == "" {
					return Config{}, errInvalidConfig("line %d: field without a category", lineNumber)
				}
				category := config.Categories[currentCategory]
				nextReading, err := applyCategoryField(&category, trimmed, lineNumber)
				if err != nil {
					return Config{}, err
				}
				config.Categories[currentCategory] = category
				readingList = nextReading
			case "profiles":
				if currentProfile == "" {
					return Config{}, errInvalidConfig("line %d: field without a profile", lineNumber)
				}
				if trimmed == "categories:" {
					profile := config.Profiles[currentProfile]
					if profile.Categories == nil {
						profile.Categories = make(map[string]ConfiguredCategory)
						config.Profiles[currentProfile] = profile
					}
					profileCategories = true
					currentCategory = ""
					readingList = ""
					continue
				}
				profileCategories = false
				profile := config.Profiles[currentProfile]
				nextReading, err := applyProfileField(&profile, trimmed, lineNumber)
				if err != nil {
					return Config{}, err
				}
				config.Profiles[currentProfile] = profile
				readingList = nextReading
			default:
				return Config{}, errInvalidConfig("line %d: unexpected indentation", lineNumber)
			}
		case 6:
			if currentSection == "categories" {
				if currentCategory == "" || !strings.HasPrefix(trimmed, "-") {
					return Config{}, errInvalidConfig("line %d: unexpected indentation", lineNumber)
				}
				category := config.Categories[currentCategory]
				if err := appendCategoryListItem(&category, readingList, trimmed, lineNumber); err != nil {
					return Config{}, err
				}
				config.Categories[currentCategory] = category
				continue
			}

			if currentSection != "profiles" || currentProfile == "" {
				return Config{}, errInvalidConfig("line %d: unexpected indentation", lineNumber)
			}
			if profileCategories {
				if !strings.HasSuffix(trimmed, ":") {
					return Config{}, errInvalidConfig("line %d: expected a profile category name", lineNumber)
				}
				currentCategory = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
				if currentCategory == "" {
					return Config{}, errInvalidConfig("line %d: category name cannot be empty", lineNumber)
				}
				profile := config.Profiles[currentProfile]
				if profile.Categories == nil {
					profile.Categories = make(map[string]ConfiguredCategory)
				}
				profile.Categories[currentCategory] = ConfiguredCategory{}
				config.Profiles[currentProfile] = profile
				readingList = ""
				continue
			}

			if !strings.HasPrefix(trimmed, "-") {
				return Config{}, errInvalidConfig("line %d: expected a list item", lineNumber)
			}
			profile := config.Profiles[currentProfile]
			if err := appendProfileListItem(&profile, readingList, trimmed, lineNumber); err != nil {
				return Config{}, err
			}
			config.Profiles[currentProfile] = profile
		case 8:
			if currentSection != "profiles" || currentProfile == "" || !profileCategories || currentCategory == "" {
				return Config{}, errInvalidConfig("line %d: unexpected indentation", lineNumber)
			}
			profile := config.Profiles[currentProfile]
			category := profile.Categories[currentCategory]
			nextReading, err := applyCategoryField(&category, trimmed, lineNumber)
			if err != nil {
				return Config{}, err
			}
			profile.Categories[currentCategory] = category
			config.Profiles[currentProfile] = profile
			readingList = nextReading
		case 10:
			if currentSection != "profiles" || currentProfile == "" || !profileCategories || currentCategory == "" || !strings.HasPrefix(trimmed, "-") {
				return Config{}, errInvalidConfig("line %d: unexpected indentation", lineNumber)
			}
			profile := config.Profiles[currentProfile]
			category := profile.Categories[currentCategory]
			if err := appendCategoryListItem(&category, readingList, trimmed, lineNumber); err != nil {
				return Config{}, err
			}
			profile.Categories[currentCategory] = category
			config.Profiles[currentProfile] = profile
		default:
			return Config{}, errInvalidConfig("line %d: unsupported indentation", lineNumber)
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	if len(config.Categories) == 0 && len(config.Profiles) == 0 {
		return Config{}, errInvalidConfig("missing categories or profiles block")
	}

	return config, nil
}

func applyCategoryField(category *ConfiguredCategory, trimmed string, lineNumber int) (string, error) {
	key, value, ok := strings.Cut(trimmed, ":")
	if !ok {
		return "", errInvalidConfig("line %d: expected key/value pair", lineNumber)
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	switch key {
	case "extensions":
		if value == "" {
			category.Extensions = nil
			return "category.extensions", nil
		}
		extensions, err := parseInlineYAMLList(value)
		if err != nil {
			return "", errInvalidConfig("line %d: %v", lineNumber, err)
		}
		category.Extensions = append([]string(nil), extensions...)
		return "", nil
	case "destination":
		category.Destination = parseYAMLScalar(value)
		return "", nil
	default:
		return "", errInvalidConfig("line %d: unknown key %q", lineNumber, key)
	}
}

func applyProfileField(profile *ConfiguredProfile, trimmed string, lineNumber int) (string, error) {
	key, value, ok := strings.Cut(trimmed, ":")
	if !ok {
		return "", errInvalidConfig("line %d: expected key/value pair", lineNumber)
	}

	key = normalizeProfileKey(key)
	value = strings.TrimSpace(value)

	switch key {
	case "backup":
		parsed, err := parseYAMLBool(value, lineNumber)
		if err != nil {
			return "", err
		}
		profile.Backup = &parsed
	case "by_date":
		parsed, err := parseYAMLBool(value, lineNumber)
		if err != nil {
			return "", err
		}
		profile.ByDate = &parsed
	case "by_size":
		parsed, err := parseYAMLBool(value, lineNumber)
		if err != nil {
			return "", err
		}
		profile.BySize = &parsed
	case "large_files_over":
		profile.LargeFilesOver = parseYAMLScalar(value)
	case "duplicate_strategy":
		profile.DuplicateStrategy = parseYAMLScalar(value)
	case "include":
		values, nextReading, err := parseYAMLStringList(value, lineNumber, "profile.include")
		if err != nil {
			return "", err
		}
		profile.Include = values
		return nextReading, nil
	case "exclude":
		values, nextReading, err := parseYAMLStringList(value, lineNumber, "profile.exclude")
		if err != nil {
			return "", err
		}
		profile.Exclude = values
		return nextReading, nil
	case "ignore_patterns":
		values, nextReading, err := parseYAMLStringList(value, lineNumber, "profile.ignore_patterns")
		if err != nil {
			return "", err
		}
		profile.IgnorePatterns = values
		return nextReading, nil
	default:
		return "", errInvalidConfig("line %d: unknown profile key %q", lineNumber, key)
	}

	return "", nil
}

func appendCategoryListItem(category *ConfiguredCategory, readingList, trimmed string, lineNumber int) error {
	if readingList != "category.extensions" {
		return errInvalidConfig("line %d: unexpected list item", lineNumber)
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
	category.Extensions = append(category.Extensions, parseYAMLScalar(value))
	return nil
}

func appendProfileListItem(profile *ConfiguredProfile, readingList, trimmed string, lineNumber int) error {
	value := parseYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
	switch readingList {
	case "profile.include":
		profile.Include = append(profile.Include, value)
	case "profile.exclude":
		profile.Exclude = append(profile.Exclude, value)
	case "profile.ignore_patterns":
		profile.IgnorePatterns = append(profile.IgnorePatterns, value)
	default:
		return errInvalidConfig("line %d: unexpected list item", lineNumber)
	}
	return nil
}

func parseYAMLStringList(value string, lineNumber int, readingList string) ([]string, string, error) {
	if value == "" {
		return nil, readingList, nil
	}
	values, err := parseInlineYAMLList(value)
	if err != nil {
		return nil, "", errInvalidConfig("line %d: %v", lineNumber, err)
	}
	return values, "", nil
}

func parseYAMLBool(value string, lineNumber int) (bool, error) {
	switch strings.ToLower(parseYAMLScalar(value)) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	default:
		return false, errInvalidConfig("line %d: expected a boolean value", lineNumber)
	}
}

func parseInlineYAMLList(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("list values must be written as [a, b, c]")
	}

	value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, parseYAMLScalar(part))
	}
	return out, nil
}

func parseYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func normalizeProfileKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

func mergeConfigs(base, overlay Config) Config {
	if base.Categories == nil {
		base.Categories = make(map[string]ConfiguredCategory)
	}
	if base.Profiles == nil {
		base.Profiles = make(map[string]ConfiguredProfile)
	}

	for name, category := range overlay.Categories {
		base.Categories[name] = category
	}
	for name, profile := range overlay.Profiles {
		base.Profiles[name] = mergeProfiles(base.Profiles[name], profile)
	}

	return base
}

func mergeProfiles(base, overlay ConfiguredProfile) ConfiguredProfile {
	merged := base
	if merged.Categories == nil {
		merged.Categories = make(map[string]ConfiguredCategory)
	}
	for name, category := range overlay.Categories {
		merged.Categories[name] = category
	}
	if overlay.Backup != nil {
		merged.Backup = overlay.Backup
	}
	if overlay.ByDate != nil {
		merged.ByDate = overlay.ByDate
	}
	if overlay.BySize != nil {
		merged.BySize = overlay.BySize
	}
	if overlay.LargeFilesOver != "" {
		merged.LargeFilesOver = overlay.LargeFilesOver
	}
	if len(overlay.Include) > 0 {
		merged.Include = append([]string(nil), overlay.Include...)
	}
	if len(overlay.Exclude) > 0 {
		merged.Exclude = append([]string(nil), overlay.Exclude...)
	}
	if len(overlay.IgnorePatterns) > 0 {
		merged.IgnorePatterns = append([]string(nil), overlay.IgnorePatterns...)
	}
	if overlay.DuplicateStrategy != "" {
		merged.DuplicateStrategy = overlay.DuplicateStrategy
	}
	return merged
}

func cloneCategories(src map[string]ConfiguredCategory) map[string]ConfiguredCategory {
	dst := make(map[string]ConfiguredCategory, len(src))
	for name, category := range src {
		dst[name] = category
	}
	return dst
}

func errInvalidConfig(format string, args ...any) error {
	return fmt.Errorf("invalid config: "+format, args...)
}
