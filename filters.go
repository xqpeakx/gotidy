package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultLargeFileThreshold = 100 * 1024 * 1024

type DuplicateStrategy string

const (
	DuplicateSkip      DuplicateStrategy = "skip"
	DuplicateRename    DuplicateStrategy = "rename"
	DuplicateOverwrite DuplicateStrategy = "overwrite"
)

func parsePatternList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, part := range parts {
		pattern := strings.TrimSpace(part)
		if pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	return patterns
}

func loadIgnorePatterns(root, explicitPath string) ([]string, string, error) {
	path := explicitPath
	if path == "" {
		path = filepath.Join(root, ".gotidyignore")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicitPath == "" && os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("cannot read ignore file %q: %w", path, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	patterns := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("cannot read ignore file %q: %w", path, err)
	}

	return patterns, path, nil
}

func matchesAnyPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}

		candidate := strings.TrimSuffix(pattern, "/")
		if ok, _ := filepath.Match(candidate, name); ok {
			return true
		}
		if candidate == name {
			return true
		}
	}
	return false
}

func parseSize(value string) (int64, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return 0, fmt.Errorf("size cannot be empty")
	}

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for _, suffix := range []string{"TB", "GB", "MB", "KB", "B"} {
		if strings.HasSuffix(value, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, suffix))
			return parseSizedNumber(number, multipliers[suffix])
		}
	}

	return parseSizedNumber(value, 1)
}

func parseSizedNumber(value string, multiplier int64) (int64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q", value)
	}
	if f < 0 {
		return 0, fmt.Errorf("size cannot be negative")
	}
	return int64(f * float64(multiplier)), nil
}

func formatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}

	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(size)
	for _, unit := range units {
		value /= 1024
		if value < 1024 || unit == units[len(units)-1] {
			if value >= 100 {
				return fmt.Sprintf("%.0f %s", value, unit)
			}
			if value >= 10 {
				return fmt.Sprintf("%.1f %s", value, unit)
			}
			return fmt.Sprintf("%.2f %s", value, unit)
		}
	}

	return fmt.Sprintf("%d B", size)
}
