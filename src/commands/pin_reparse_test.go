package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefuseDetectFileReparseAllowsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".nvmrc")
	if err := refuseDetectFileReparse(path); err != nil {
		t.Fatalf("missing file: %v", err)
	}
}

func TestRefuseDetectFileReparseAllowsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".nvmrc")
	if err := os.WriteFile(path, []byte("20.11.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := refuseDetectFileReparse(path); err != nil {
		t.Fatalf("regular file: %v", err)
	}
}

func TestRefuseDetectFileReparseRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	link := filepath.Join(dir, ".nvmrc")
	if err := os.WriteFile(target, []byte("token=leak\n"), 0o644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}
	err := refuseDetectFileReparse(link)
	if err == nil {
		t.Fatal("expected refusal for symlink detect file")
	}
	if !strings.Contains(err.Error(), "symbolic link or reparse point") {
		t.Fatalf("error = %v, want reparse refusal", err)
	}
}
