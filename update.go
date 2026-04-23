package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	modulePath   = "github.com/xqpeakx/gotidy"
	updateTarget = modulePath + "@main"
)

var selfUpdate = runSelfUpdate

func runSelfUpdate(stdout, stderr io.Writer) error {
	currentPath, err := currentExecutablePath()
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "gotidy-update-*")
	if err != nil {
		return fmt.Errorf("cannot create a temporary update directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("go", "install", updateTarget)
	cmd.Env = append(os.Environ(), "GOPROXY=direct")
	cmd.Env = append(cmd.Env, "GOBIN="+tempDir)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("cannot update gotidy: the go command was not found")
		}
		return fmt.Errorf("cannot update gotidy with %q: %w", cmd.String(), err)
	}

	newBinaryPath := filepath.Join(tempDir, binaryName())
	if _, err := os.Stat(newBinaryPath); err != nil {
		return fmt.Errorf("update finished, but the new binary was not found at %q: %w", newBinaryPath, err)
	}

	if err := replaceCurrentExecutable(newBinaryPath, currentPath); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Updated gotidy at %s.\n", currentPath)
	return nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "gotidy.exe"
	}
	return "gotidy"
}

func currentExecutablePath() (string, error) {
	currentPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine the current gotidy path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(currentPath)
	if err == nil {
		currentPath = resolvedPath
	}

	return filepath.Clean(currentPath), nil
}

func replaceCurrentExecutable(srcPath, dstPath string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("--update is not supported on Windows yet")
	}

	stagedPath := dstPath + ".tmp-update"
	_ = os.Remove(stagedPath)
	defer os.Remove(stagedPath)

	if err := moveOrCopyFile(srcPath, stagedPath); err != nil {
		return fmt.Errorf("cannot stage the updated binary for %q: %w", dstPath, err)
	}
	if err := os.Rename(stagedPath, dstPath); err != nil {
		return fmt.Errorf("cannot replace the current gotidy binary at %q: %w", dstPath, err)
	}

	return nil
}

func moveOrCopyFile(srcPath, dstPath string) error {
	if err := os.Rename(srcPath, dstPath); err == nil {
		return nil
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(dstFile, srcFile)
	closeErr := dstFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chmod(dstPath, srcInfo.Mode().Perm()); err != nil {
		return err
	}
	return nil
}
