package link

import (
	"os"
	"path/filepath"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

const linkTestRegistryRoot = "HKCU/Software/NVMTest/link_require_symlink"

func TestMain(m *testing.M) {
	prefs.ROOT = linkTestRegistryRoot
	prefs.ROOTS = []string{prefs.ROOT}
	code := m.Run()
	os.Exit(code)
}

func setRequireActivationSymlinks(t *testing.T, value bool) {
	t.Helper()
	if err := settings.Put("require_activation_symlinks", value); err != nil {
		t.Fatalf("Put(require_activation_symlinks) error = %v", err)
	}
	settings.Load(true)
}

func TestLinkRequireActivationSymlinksCreatesSymlink(t *testing.T) {
	setRequireActivationSymlinks(t, true)
	t.Cleanup(func() { setRequireActivationSymlinks(t, false) })

	root := t.TempDir()
	source := filepath.Join(root, "target")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "node.exe"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile(node.exe) error = %v", err)
	}
	dest := filepath.Join(root, ".nodejs")

	if err := Link(source, dest); err != nil {
		t.Fatalf("Link() error = %v", err)
	}
	if IsMountPointJunction(dest) {
		t.Fatal("expected symlink, got mount-point junction")
	}
	info, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("Lstat(dest) error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dest mode = %v, want symlink", info.Mode())
	}
}

func TestLinkDefaultCreatesJunctionWhenPossible(t *testing.T) {
	setRequireActivationSymlinks(t, false)

	root := t.TempDir()
	source := filepath.Join(root, "target")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("MkdirAll(source) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "node.exe"), []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile(node.exe) error = %v", err)
	}
	dest := filepath.Join(root, ".nodejs")

	if err := Link(source, dest); err != nil {
		t.Fatalf("Link() error = %v", err)
	}
	if !IsMountPointJunction(dest) {
		t.Fatal("expected mount-point junction when RequireActivationSymlinks is false")
	}
}
