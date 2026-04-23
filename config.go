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
}

type ConfiguredCategory struct {
	Extensions  []string `json:"extensions"`
	Destination string   `json:"destination"`
}

type LoadedConfig struct {
	Path   string
	Config Config
}

var defaultConfigNames = []string{
	".gotidy.yaml",
	".gotidy.yml",
	".gotidy.json",
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
		return &LoadedConfig{
			Path:   path,
			Config: config,
		}, nil
	}

	return nil, nil
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
	return config, nil
}

func parseYAMLConfig(data []byte) (Config, error) {
	config := Config{Categories: make(map[string]ConfiguredCategory)}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	var (
		lineNumber        int
		inCategoriesBlock bool
		currentCategory   string
		readingExtensions bool
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

		switch {
		case indent == 0:
			if trimmed != "categories:" {
				return Config{}, errInvalidConfig("line %d: expected \"categories:\"", lineNumber)
			}
			inCategoriesBlock = true
			currentCategory = ""
			readingExtensions = false
		case indent == 2:
			if !inCategoriesBlock || !strings.HasSuffix(trimmed, ":") {
				return Config{}, errInvalidConfig("line %d: expected a category name", lineNumber)
			}
			currentCategory = strings.TrimSpace(strings.TrimSuffix(trimmed, ":"))
			if currentCategory == "" {
				return Config{}, errInvalidConfig("line %d: category name cannot be empty", lineNumber)
			}
			config.Categories[currentCategory] = ConfiguredCategory{}
			readingExtensions = false
		case indent == 4:
			if currentCategory == "" {
				return Config{}, errInvalidConfig("line %d: field without a category", lineNumber)
			}
			key, value, ok := strings.Cut(trimmed, ":")
			if !ok {
				return Config{}, errInvalidConfig("line %d: expected key/value pair", lineNumber)
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)

			category := config.Categories[currentCategory]
			switch key {
			case "extensions":
				if value == "" {
					category.Extensions = nil
					config.Categories[currentCategory] = category
					readingExtensions = true
					continue
				}
				extensions, err := parseInlineYAMLList(value)
				if err != nil {
					return Config{}, errInvalidConfig("line %d: %v", lineNumber, err)
				}
				category.Extensions = append([]string(nil), extensions...)
				config.Categories[currentCategory] = category
				readingExtensions = false
			case "destination":
				category.Destination = parseYAMLScalar(value)
				config.Categories[currentCategory] = category
				readingExtensions = false
			default:
				return Config{}, errInvalidConfig("line %d: unknown key %q", lineNumber, key)
			}
		case indent == 6:
			if currentCategory == "" || !readingExtensions || !strings.HasPrefix(trimmed, "-") {
				return Config{}, errInvalidConfig("line %d: unexpected list item", lineNumber)
			}
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			category := config.Categories[currentCategory]
			category.Extensions = append(category.Extensions, parseYAMLScalar(value))
			config.Categories[currentCategory] = category
		default:
			return Config{}, errInvalidConfig("line %d: unsupported indentation", lineNumber)
		}
	}

	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	if !inCategoriesBlock {
		return Config{}, errInvalidConfig("missing categories block")
	}

	return config, nil
}

func parseInlineYAMLList(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("extensions must be written as [a, b, c]")
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

func errInvalidConfig(format string, args ...any) error {
	return fmt.Errorf("invalid config: "+format, args...)
}
