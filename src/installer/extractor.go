package installer

import (
	"bufio"
	"common/fs"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
)

func find7zExe() string {
	if path, err := exec.LookPath("7z.exe"); err == nil {
		return path
	}
	candidates := []string{
		`C:\Program Files\7-Zip\7z.exe`,
		`C:\Program Files (x86)\7-Zip\7z.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func extract7zNative(ctx context.Context, szExe, archive, destination string) error {
	var err error
	parent := filepath.Dir(destination)
	tmpDir, err := os.MkdirTemp(parent, ".nvm-extract-*")
	if err != nil {
		return err
	}
	fs.SetHidden(tmpDir)
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
			return os.Rename(filepath.Join(tmpDir, e.Name()), destination)
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
	fs.SetHidden(destination)
	defer fs.ClearHidden(destination)

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
	return nil
}

func extract7zFile(f *sevenzip.File, destination, relPath string) error {
	cleanDest := filepath.Clean(destination)
	targetPath := filepath.Join(cleanDest, relPath)
	cleanTarget := filepath.Clean(targetPath)

	if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
		return fmt.Errorf("invalid archive path: %s", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(cleanTarget, 0755)
	}

	if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
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
