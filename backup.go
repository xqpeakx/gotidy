package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func createBackup(root string) (string, error) {
	name := fmt.Sprintf(".gotidy-backup-%s.zip", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(root, name)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("cannot create backup %q: %w", path, err)
	}

	writer := zip.NewWriter(file)
	closeWithError := func(closeErr error) error {
		_ = writer.Close()
		_ = file.Close()
		_ = os.Remove(path)
		return closeErr
	}

	err = filepath.WalkDir(root, func(walkPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if walkPath == path {
			return nil
		}

		rel, err := filepath.Rel(root, walkPath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(rel) + "/"
			_, err = writer.CreateHeader(header)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate

		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		src, err := os.Open(walkPath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entryWriter, src)
		closeErr := src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	})
	if err != nil {
		return "", closeWithError(fmt.Errorf("cannot create backup %q: %w", path, err))
	}

	if err := writer.Close(); err != nil {
		return "", closeWithError(fmt.Errorf("cannot finish backup %q: %w", path, err))
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("cannot finalize backup %q: %w", path, err)
	}

	return path, nil
}
