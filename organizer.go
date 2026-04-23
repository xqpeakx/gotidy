package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errInteractiveQuit = errors.New("interactive organize cancelled")

type Options struct {
	DryRun             bool
	Verbose            bool
	Stats              bool
	Interactive        bool
	Backup             bool
	ByDate             bool
	BySize             bool
	CollisionStrategy  DuplicateStrategy
	LargeFileThreshold int64
	IncludePatterns    []string
	ExcludePatterns    []string
	IgnorePatterns     []string
	Resolver           CategoryResolver
	ConfigPath         string
	IgnoreFilePath     string
	Logf               func(format string, args ...any)
	Prompt             func(prompt string) (string, error)
}

type Summary struct {
	Examined        int
	Moved           int
	Renamed         int
	Overwritten     int
	Skipped         int
	Filtered        int
	TotalBytes      int64
	ByCategory      map[string]int
	ByCategoryBytes map[string]int64
	BackupPath      string
	ConfigPath      string
	IgnoreFilePath  string
}

type movePlan struct {
	Name            string
	SourcePath      string
	DestinationPath string
	DestinationRel  string
	Category        string
	Bytes           int64
	Renamed         bool
	Overwrite       bool
}

func Organize(dir string, opts Options) (Summary, error) {
	opts = normalizeOptions(opts)
	summary := Summary{
		ByCategory:      make(map[string]int),
		ByCategoryBytes: make(map[string]int64),
		ConfigPath:      opts.ConfigPath,
		IgnoreFilePath:  opts.IgnoreFilePath,
	}
	moves := make([]moveRecord, 0)

	if err := ensureDirectory(dir); err != nil {
		return summary, err
	}

	if !opts.DryRun && opts.Backup {
		backupPath, err := createBackup(dir)
		if err != nil {
			return summary, err
		}
		summary.BackupPath = backupPath
		opts.logf("created backup %s", filepath.Base(backupPath))
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return summary, fmt.Errorf("cannot read %q: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		summary.Examined++

		filtered, reason := shouldFilterEntry(entry, opts)
		if filtered {
			opts.logf("left %s where it is (%s)", name, reason)
			summary.Filtered++
			continue
		}

		if len(opts.IncludePatterns) > 0 && !matchesAnyPattern(name, opts.IncludePatterns) {
			opts.logf("left %s where it is (does not match --include)", name)
			summary.Filtered++
			continue
		}
		if matchesAnyPattern(name, opts.ExcludePatterns) {
			opts.logf("left %s where it is (matched --exclude)", name)
			summary.Filtered++
			continue
		}
		if matchesAnyPattern(name, opts.IgnorePatterns) {
			opts.logf("left %s where it is (matched ignore pattern)", name)
			summary.Filtered++
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot stat %q: %w", filepath.Join(dir, name), err))
		}

		plan, skipped, err := buildMovePlan(dir, info, opts)
		if err != nil {
			return summary, saveOrganizeStateOnError(dir, moves, err)
		}
		if skipped {
			summary.Skipped++
			continue
		}

		if opts.DryRun {
			if plan.Overwrite {
				opts.logf("[dry-run] would overwrite %s with %s", filepath.ToSlash(plan.DestinationRel), name)
			} else if plan.Renamed {
				opts.logf("[dry-run] would move %s to %s", name, filepath.ToSlash(plan.DestinationRel))
			} else {
				opts.logf("[dry-run] would move %s to %s", name, filepath.ToSlash(plan.DestinationRel))
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(plan.DestinationPath), 0o755); err != nil {
				return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot create %q: %w", filepath.Dir(plan.DestinationPath), err))
			}
			if plan.Overwrite {
				if err := os.Remove(plan.DestinationPath); err != nil && !errors.Is(err, os.ErrNotExist) {
					return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot remove %q before overwrite: %w", plan.DestinationPath, err))
				}
			}
			if err := os.Rename(plan.SourcePath, plan.DestinationPath); err != nil {
				return summary, saveOrganizeStateOnError(dir, moves, fmt.Errorf("cannot move %q to %q: %w", plan.SourcePath, plan.DestinationPath, err))
			}
			switch {
			case plan.Overwrite:
				opts.logf("overwrote %s with %s", filepath.ToSlash(plan.DestinationRel), name)
			case plan.Renamed:
				opts.logf("moved %s to %s", name, filepath.ToSlash(plan.DestinationRel))
			default:
				opts.logf("moved %s to %s", name, filepath.ToSlash(plan.DestinationRel))
			}
			moves = append(moves, moveRecord{
				From:     filepath.ToSlash(plan.Name),
				To:       filepath.ToSlash(plan.DestinationRel),
				Category: plan.Category,
			})
		}

		summary.Moved++
		if plan.Renamed {
			summary.Renamed++
		}
		if plan.Overwrite {
			summary.Overwritten++
		}
		summary.TotalBytes += plan.Bytes
		summary.ByCategory[plan.Category]++
		summary.ByCategoryBytes[plan.Category] += plan.Bytes
	}

	if err := saveOrganizeState(dir, moves); err != nil {
		return summary, err
	}

	return summary, nil
}

func normalizeOptions(opts Options) Options {
	if opts.Resolver.byExtension == nil {
		opts.Resolver = DefaultCategoryResolver()
	}
	if opts.CollisionStrategy == "" {
		opts.CollisionStrategy = DuplicateSkip
	}
	if opts.LargeFileThreshold <= 0 {
		opts.LargeFileThreshold = defaultLargeFileThreshold
	}
	return opts
}

func buildMovePlan(root string, info os.FileInfo, opts Options) (movePlan, bool, error) {
	name := info.Name()
	rule := opts.Resolver.Resolve(name)
	relativeDir := rule.Destination
	if relativeDir == "" {
		relativeDir = rule.Name
	}

	if opts.BySize && info.Size() >= opts.LargeFileThreshold {
		relativeDir = filepath.Join("large_files", relativeDir)
	}
	if opts.ByDate {
		year, month, _ := info.ModTime().Date()
		relativeDir = filepath.Join(relativeDir, fmt.Sprintf("%04d", year), fmt.Sprintf("%02d", int(month)))
	}

	destinationDir, err := safeJoin(root, relativeDir)
	if err != nil {
		return movePlan{}, false, fmt.Errorf("invalid destination %q for %s: %w", relativeDir, name, err)
	}

	targetName := name
	targetPath := filepath.Join(destinationDir, targetName)
	exists, err := fileExists(targetPath)
	if err != nil {
		return movePlan{}, false, fmt.Errorf("cannot stat %q: %w", targetPath, err)
	}

	renamed := false
	overwrite := false
	if exists {
		if opts.Interactive {
			choice, err := promptCollisionChoice(opts, name, filepath.ToSlash(filepath.Join(relativeDir, targetName)))
			if err != nil {
				return movePlan{}, false, err
			}
			switch choice {
			case DuplicateSkip:
				opts.logf("left %s where it is (skipped at prompt)", name)
				return movePlan{}, true, nil
			case DuplicateRename:
				targetName, targetPath, err = resolveRenamedDestination(destinationDir, name)
				if err != nil {
					return movePlan{}, false, fmt.Errorf("cannot resolve a renamed destination for %q: %w", name, err)
				}
				renamed = true
			case DuplicateOverwrite:
				overwrite = true
			default:
				return movePlan{}, false, fmt.Errorf("unknown duplicate choice %q", choice)
			}
		} else {
			switch opts.CollisionStrategy {
			case DuplicateSkip:
				opts.logf("left %s where it is (already exists at %s)", name, filepath.ToSlash(filepath.Join(relativeDir, targetName)))
				return movePlan{}, true, nil
			case DuplicateRename:
				targetName, targetPath, err = resolveRenamedDestination(destinationDir, name)
				if err != nil {
					return movePlan{}, false, fmt.Errorf("cannot resolve a renamed destination for %q: %w", name, err)
				}
				renamed = true
			case DuplicateOverwrite:
				overwrite = true
			default:
				return movePlan{}, false, fmt.Errorf("unknown duplicate strategy %q", opts.CollisionStrategy)
			}
		}
	}

	destinationRel := filepath.Join(relativeDir, targetName)
	if opts.Interactive && !exists {
		ok, err := promptMoveChoice(opts, name, filepath.ToSlash(destinationRel))
		if err != nil {
			return movePlan{}, false, err
		}
		if !ok {
			opts.logf("left %s where it is (skipped at prompt)", name)
			return movePlan{}, true, nil
		}
	}

	return movePlan{
		Name:            name,
		SourcePath:      filepath.Join(root, name),
		DestinationPath: targetPath,
		DestinationRel:  destinationRel,
		Category:        rule.Name,
		Bytes:           info.Size(),
		Renamed:         renamed,
		Overwrite:       overwrite,
	}, false, nil
}

func resolveRenamedDestination(destinationDir, name string) (string, string, error) {
	for i := 1; ; i++ {
		candidateName := collisionName(name, i)
		candidatePath := filepath.Join(destinationDir, candidateName)
		exists, err := fileExists(candidatePath)
		if err != nil {
			return "", "", err
		}
		if !exists {
			return candidateName, candidatePath, nil
		}
	}
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

func shouldFilterEntry(entry os.DirEntry, opts Options) (bool, string) {
	switch {
	case entry.IsDir():
		return true, "directory"
	case len(entry.Name()) > 0 && entry.Name()[0] == '.':
		return true, "hidden file"
	case !entry.Type().IsRegular():
		return true, "special file"
	default:
		return false, ""
	}
}

func (o Options) logf(format string, args ...any) {
	if !o.Verbose || o.Logf == nil {
		return
	}
	o.Logf(format, args...)
}

func promptMoveChoice(opts Options, name, destination string) (bool, error) {
	if opts.Prompt == nil {
		return false, fmt.Errorf("--interactive requires a prompt reader")
	}

	for {
		answer, err := opts.Prompt(fmt.Sprintf("move %s -> %s? [y/N/q]: ", name, destination))
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "n", "no":
			return false, nil
		case "y", "yes":
			return true, nil
		case "q", "quit":
			return false, errInteractiveQuit
		}
	}
}

func promptCollisionChoice(opts Options, name, destination string) (DuplicateStrategy, error) {
	if opts.Prompt == nil {
		return "", fmt.Errorf("--interactive requires a prompt reader")
	}

	defaultChoice := opts.CollisionStrategy
	if defaultChoice == "" {
		defaultChoice = DuplicateSkip
	}

	for {
		answer, err := opts.Prompt(fmt.Sprintf("collision for %s at %s [s]kip/[r]ename/[o]verwrite/[q]uit (default: %s): ", name, destination, defaultChoice))
		if err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "":
			return defaultChoice, nil
		case "s", "skip":
			return DuplicateSkip, nil
		case "r", "rename":
			return DuplicateRename, nil
		case "o", "overwrite":
			return DuplicateOverwrite, nil
		case "q", "quit":
			return "", errInteractiveQuit
		}
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func collisionName(name string, suffix int) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if base == "" {
		base = name
		ext = ""
	}
	return fmt.Sprintf("%s_%d%s", base, suffix, ext)
}
