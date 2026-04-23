package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	DryRun  bool
	Verbose bool
	Logf    func(format string, args ...any)
}

type Summary struct {
	Moved      int
	Skipped    int
	ByCategory map[string]int
}

func Organize(dir string, opts Options) (Summary, error) {
	summary := Summary{ByCategory: make(map[string]int)}
	moves := make([]moveRecord, 0)

	if err := ensureDirectory(dir); err != nil {
		return summary, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return summary, fmt.Errorf("cannot read %q: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()

		if shouldSkip(entry) {
			opts.logf("left %s where it is (directory, hidden file, or special file)", name)
			continue
		}

		category := CategoryFor(name)
		srcPath := filepath.Join(dir, name)
		dstDir := filepath.Join(dir, category)
		dstPath := filepath.Join(dstDir, name)

		if _, err := os.Stat(dstPath); err == nil {
			opts.logf("left %s where it is (already exists in %s/)", name, category)
			summary.Skipped++
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot stat %q: %w", dstPath, err))
		}

		if opts.DryRun {
			opts.logf("[dry-run] would move %s to %s/", name, category)
		} else {
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot create %q: %w", dstDir, err))
			}
			if err := os.Rename(srcPath, dstPath); err != nil {
				return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot move %q to %q: %w", srcPath, dstPath, err))
			}
			opts.logf("moved %s to %s/", name, category)
			moves = append(moves, moveRecord{
				From: filepath.ToSlash(name),
				To:   filepath.ToSlash(filepath.Join(category, name)),
			})
		}

		summary.Moved++
		summary.ByCategory[category]++
	}

	if err := saveOrganizeState(dir, moves); err != nil {
		return summary, err
	}

	return summary, nil
}

func ensureDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot access %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}
	return nil
}

func shouldSkip(entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
		return true
	}
	if !entry.Type().IsRegular() {
		return true
	}
	return false
}

func (o Options) logf(format string, args ...any) {
	if !o.Verbose || o.Logf == nil {
		return
	}
	o.Logf(format, args...)
}
