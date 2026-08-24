package installer

import (
	"bufio"
	commonfs "common/fs"
	"context"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
)

var pinned7zCandidates = []string{
	`C:\Program Files\7-Zip\7z.exe`,
	`C:\Program Files (x86)\7-Zip\7z.exe`,
}

// archivePathProbeRoot is a stable root for zip-slip checks independent of destination.
const archivePathProbeRoot = `C:\nvm-extract-validation`

func find7zExe() string {
	for _, candidate := range pinned7zCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func extractPathWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	return strings.HasPrefix(target, root+string(os.PathSeparator))
}

func validateRelPathUnderRoot(root, relPath, rawName string) error {
	if relPath == "" {
		return nil
	}
	cleanRoot := filepath.Clean(root)
	target := filepath.Clean(filepath.Join(cleanRoot, relPath))
	if !extractPathWithinRoot(cleanRoot, target) {
		return fmt.Errorf("invalid archive path: %s", rawName)
	}
	return nil
}

func validateArchivePaths(archive string) error {
	r, err := sevenzip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		relPath := archiveRelativePath(f.Name)
		if err := validateRelPathUnderRoot(archivePathProbeRoot, relPath, f.Name); err != nil {
			return err
		}
	}
	return nil
}

func validateExtractTree(root string) error {
	cleanRoot := filepath.Clean(root)
	info, err := os.Lstat(cleanRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("extracted path escapes destination: %s", cleanRoot)
	}

	return filepath.WalkDir(cleanRoot, func(path string, entry iofs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !extractPathWithinRoot(cleanRoot, path) {
			return fmt.Errorf("extracted path escapes destination: %s", path)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink not allowed in extracted archive: %s", path)
		}
		return nil
	})
}

func extract7zNative(ctx context.Context, szExe, archive, destination string) error {
	var err error
	parent := filepath.Dir(destination)
	tmpDir, err := os.MkdirTemp(parent, ".nvm-extract-*")
	if err != nil {
		return err
	}
	commonfs.SetHidden(tmpDir)
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, szExe, "x", archive, "-o"+tmpDir, "-y", "-bb0", "-bsp0", "-bso0")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err = cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("7z extraction failed: %w", err)
	}

	if err = validateExtractTree(tmpDir); err != nil {
		return fmt.Errorf("native extraction failed path validation: %w", err)
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err = os.Stat(destination); err == nil {
				if err = os.RemoveAll(destination); err != nil {
					return err
				}
			}
			if err = os.Rename(filepath.Join(tmpDir, e.Name()), destination); err != nil {
				return err
			}
			if err = validateExtractTree(destination); err != nil {
				_ = os.RemoveAll(destination)
				return fmt.Errorf("native extraction failed path validation: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("no directory found in extracted archive")
}

func extract7zGo(ctx context.Context, archive, destination string) error {
	r, err := sevenzip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	if err = os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	if err = commonfs.AssertNoReparseBetween(destination, destination); err != nil {
		return err
	}
	commonfs.SetHidden(destination)
	defer commonfs.ClearHidden(destination)

	for _, f := range r.File {
		if ctx.Err() != nil {
			return context.Canceled
		}
		relPath := archiveRelativePath(f.Name)
		if relPath == "" {
			continue
		}
		if err = extract7zFile(f, destination, relPath); err != nil {
			return err
		}
	}
	return validateExtractTree(destination)
}

func extract7zFile(f *sevenzip.File, destination, relPath string) error {
	cleanDest := filepath.Clean(destination)
	if err := validateRelPathUnderRoot(cleanDest, relPath, f.Name); err != nil {
		return err
	}
	cleanTarget := filepath.Clean(filepath.Join(cleanDest, relPath))

	if f.FileInfo().IsDir() {
		if err := commonfs.AssertNoReparseBetween(cleanDest, cleanTarget); err != nil {
			return err
		}
		return os.MkdirAll(cleanTarget, 0755)
	}

	parent := filepath.Dir(cleanTarget)
	if err := commonfs.AssertNoReparseBetween(cleanDest, parent); err != nil {
		return err
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(cleanTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	const bufSize = 1 << 20
	buf := make([]byte, bufSize)
	bw := bufio.NewWriterSize(out, bufSize)

	if _, err = io.CopyBuffer(bw, rc, buf); err != nil {
		return err
	}
	return bw.Flush()
}

func archiveRelativePath(name string) string {
	normalized := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(name)), "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" || normalized == "." {
		return ""
	}

	parts := strings.Split(normalized, "/")
	if strings.HasPrefix(parts[0], "node-v") {
		if len(parts) == 1 {
			return ""
		}
		normalized = strings.Join(parts[1:], "/")
	}

	return filepath.FromSlash(normalized)
}

func extract7z(ctx context.Context, archive, destination string, status *Status) error {
	status.Extractions++
	defer func() { status.Extractions-- }()

	if err := validateArchivePaths(archive); err != nil {
		return fmt.Errorf("archive path validation failed: %w", err)
	}

	if szExe := find7zExe(); szExe != "" {
		nativeErr := extract7zNative(ctx, szExe, archive, destination)
		if nativeErr == nil {
			return nil
		}
		if errors.Is(nativeErr, context.Canceled) {
			return context.Canceled
		}

		cleanupExtractArtifacts(filepath.Dir(destination))
		fallbackErr := extract7zGo(ctx, archive, destination)
		if fallbackErr == nil {
			return nil
		}
		if errors.Is(fallbackErr, context.Canceled) {
			return context.Canceled
		}

		status.Alert(fmt.Sprintf("Native extraction failed for %s, falling back to built-in extractor", archive))
		return fmt.Errorf("native extraction failed: %v; fallback extraction failed: %w", nativeErr, fallbackErr)
	}

	return extract7zGo(ctx, archive, destination)
}

func cleanupExtractArtifacts(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), ".nvm-extract-") {
			continue
		}
		os.RemoveAll(filepath.Join(parent, entry.Name()))
	}
}
