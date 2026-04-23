package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	undoManifestName    = ".gotidy-last-run.json"
	undoManifestVersion = 1
)

type moveRecord struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type undoManifest struct {
	Version   int          `json:"version"`
	CreatedAt string       `json:"created_at"`
	Moves     []moveRecord `json:"moves"`
}

func Undo(dir string, opts Options) (Summary, error) {
	summary := Summary{ByCategory: make(map[string]int)}

	if err := ensureDirectory(dir); err != nil {
		return summary, err
	}

	manifest, err := readUndoManifest(dir)
	if err != nil {
		return summary, err
	}

	remaining := make([]moveRecord, 0)
	for i := len(manifest.Moves) - 1; i >= 0; i-- {
		move := manifest.Moves[i]
		srcPath, dstPath, err := resolveUndoMove(dir, move)
		if err != nil {
			return summary, saveUndoStateOnError(dir, manifest.Moves, i, remaining, err)
		}

		name := filepath.Base(srcPath)
		category := filepath.Base(filepath.Dir(dstPath))

		if _, err := os.Stat(srcPath); err == nil {
			opts.logf("left %s where it is (already exists at the original location)", name)
			summary.Skipped++
			remaining = append(remaining, move)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return summary, saveUndoStateOnError(dir, manifest.Moves, i, remaining, fmt.Errorf("cannot access %q: %w", srcPath, err))
		}

		if _, err := os.Stat(dstPath); errors.Is(err, os.ErrNotExist) {
			opts.logf("left %s where it is (expected in %s/, but it is missing)", name, category)
			summary.Skipped++
			remaining = append(remaining, move)
			continue
		} else if err != nil {
			return summary, saveUndoStateOnError(dir, manifest.Moves, i, remaining, fmt.Errorf("cannot access %q: %w", dstPath, err))
		}

		if opts.DryRun {
			opts.logf("[dry-run] would move %s back from %s/", name, category)
		} else {
			if err := os.Rename(dstPath, srcPath); err != nil {
				return summary, saveUndoStateOnError(dir, manifest.Moves, i, remaining, fmt.Errorf("cannot move %q back to %q: %w", dstPath, srcPath, err))
			}
			opts.logf("moved %s back from %s/", name, category)
			_ = os.Remove(filepath.Dir(dstPath))
		}

		summary.Moved++
		summary.ByCategory[category]++
	}

	if opts.DryRun {
		return summary, nil
	}

	if len(remaining) == 0 {
		if err := removeUndoManifest(dir); err != nil {
			return summary, fmt.Errorf("restored files, but could not remove undo data in %q: %w", manifestPath(dir), err)
		}
		return summary, nil
	}

	reverseMoveRecords(remaining)
	if err := writeUndoManifest(dir, remaining); err != nil {
		return summary, fmt.Errorf("restored files, but could not update undo data in %q: %w", manifestPath(dir), err)
	}

	return summary, nil
}

func saveOrganizeState(dir string, moves []moveRecord) error {
	if len(moves) == 0 {
		return nil
	}
	if err := writeUndoManifest(dir, moves); err != nil {
		return fmt.Errorf("organized files, but could not save undo data in %q: %w", manifestPath(dir), err)
	}
	return nil
}

func saveOrganizeStateOnError(dir string, moves []moveRecord, opErr error) error {
	if len(moves) == 0 {
		return opErr
	}
	if err := writeUndoManifest(dir, moves); err != nil {
		return errors.Join(opErr, fmt.Errorf("also could not save undo data in %q: %w", manifestPath(dir), err))
	}
	return opErr
}

func saveUndoStateOnError(dir string, allMoves []moveRecord, current int, remaining []moveRecord, opErr error) error {
	pending := make([]moveRecord, 0, current+1+len(remaining))
	pending = append(pending, allMoves[:current+1]...)

	skipped := append([]moveRecord(nil), remaining...)
	reverseMoveRecords(skipped)
	pending = append(pending, skipped...)

	if len(pending) == 0 {
		return opErr
	}
	if err := writeUndoManifest(dir, pending); err != nil {
		return errors.Join(opErr, fmt.Errorf("also could not update undo data in %q: %w", manifestPath(dir), err))
	}
	return opErr
}

func readUndoManifest(dir string) (undoManifest, error) {
	path := manifestPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return undoManifest{}, fmt.Errorf("no undo data found in %q", dir)
		}
		return undoManifest{}, fmt.Errorf("cannot read undo data in %q: %w", path, err)
	}

	var manifest undoManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return undoManifest{}, fmt.Errorf("cannot parse undo data in %q: %w", path, err)
	}
	if manifest.Version != 0 && manifest.Version != undoManifestVersion {
		return undoManifest{}, fmt.Errorf("cannot use undo data in %q: unsupported version %d", path, manifest.Version)
	}
	if len(manifest.Moves) == 0 {
		return undoManifest{}, fmt.Errorf("no undo data found in %q", dir)
	}

	return manifest, nil
}

func writeUndoManifest(dir string, moves []moveRecord) error {
	manifest := undoManifest{
		Version:   undoManifestVersion,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Moves:     moves,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode undo data: %w", err)
	}
	data = append(data, '\n')

	path := manifestPath(dir)
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("cannot write undo data: %w", err)
	}
	defer os.Remove(tempPath)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("cannot replace undo data: %w", err)
	}
	return nil
}

func removeUndoManifest(dir string) error {
	if err := os.Remove(manifestPath(dir)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func manifestPath(dir string) string {
	return filepath.Join(dir, undoManifestName)
}

func resolveUndoMove(root string, move moveRecord) (string, string, error) {
	srcPath, err := safeJoin(root, move.From)
	if err != nil {
		return "", "", fmt.Errorf("invalid undo data in %q: %w", manifestPath(root), err)
	}

	dstPath, err := safeJoin(root, move.To)
	if err != nil {
		return "", "", fmt.Errorf("invalid undo data in %q: %w", manifestPath(root), err)
	}

	return srcPath, dstPath, nil
}

func safeJoin(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute path %q is not allowed", rel)
	}
	if cleaned == "." || cleaned == ".." {
		return "", fmt.Errorf("path %q is not allowed", rel)
	}
	if strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the target directory", rel)
	}

	return filepath.Join(root, cleaned), nil
}

func reverseMoveRecords(moves []moveRecord) {
	for i, j := 0, len(moves)-1; i < j; i, j = i+1, j-1 {
		moves[i], moves[j] = moves[j], moves[i]
	}
}
