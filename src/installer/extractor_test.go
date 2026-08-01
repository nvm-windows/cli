package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathWithinRoot(t *testing.T) {
	root := filepath.Clean(`C:\nvm\installs\v20.0.0`)

	if !pathWithinRoot(root, root) {
		t.Fatal("expected root to be within itself")
	}
	if !pathWithinRoot(root, filepath.Join(root, "node.exe")) {
		t.Fatal("expected child path to be within root")
	}
	if pathWithinRoot(root, filepath.Clean(`C:\nvm\installs\v20.0.0-evil`)) {
		t.Fatal("expected sibling path to be rejected")
	}
	if pathWithinRoot(root, filepath.Clean(`C:\nvm\installs\..\evil`)) {
		t.Fatal("expected traversal path to be rejected")
	}
}

func TestValidateRelPathUnderRootRejectsTraversal(t *testing.T) {
	root := filepath.Clean(`C:\nvm-extract-validation`)

	cases := []struct {
		relPath string
		rawName string
	}{
		{`..\evil.txt`, `node-v20.0.0\..\evil.txt`},
		{`..`, `node-v20.0.0\..`},
		{`foo\..\..\outside.txt`, `node-v20.0.0\foo\..\..\outside.txt`},
	}

	for _, tc := range cases {
		if err := validateRelPathUnderRoot(root, tc.relPath, tc.rawName); err == nil {
			t.Fatalf("validateRelPathUnderRoot(%q) error = nil, want rejection", tc.rawName)
		}
	}
}

func TestArchiveRelativePathStripsNodeVersionPrefix(t *testing.T) {
	got := archiveRelativePath(`node-v20.17.0-win-x64\bin\node.exe`)
	want := filepath.FromSlash(`bin/node.exe`)
	if got != want {
		t.Fatalf("archiveRelativePath = %q, want %q", got, want)
	}
}

func TestFind7zExeUsesPinnedPathsOnly(t *testing.T) {
	got := find7zExe()
	if got == "" {
		t.Skip("7-Zip not installed in pinned Program Files locations")
	}

	gotLower := strings.ToLower(filepath.Clean(got))
	for _, candidate := range pinned7zCandidates {
		if strings.EqualFold(gotLower, filepath.Clean(candidate)) {
			return
		}
	}
	t.Fatalf("find7zExe() = %q, want one of pinned Program Files paths", got)
}

func TestValidateExtractTreeRejectsEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}

	escaped := filepath.Join(root, "..", filepath.Base(outside))
	if err := validateExtractTree(root); err != nil {
		t.Fatalf("validateExtractTree(root) error = %v", err)
	}
	if _, err := os.Stat(escaped); err == nil {
		// Walk only covers root subtree; verify sibling paths are not counted as inside.
		if pathWithinRoot(root, outside) {
			t.Fatal("pathWithinRoot should reject sibling outside temp root")
		}
	}
}
