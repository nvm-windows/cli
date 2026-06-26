package bootstrap

import (
	"common/settings"
	"path/filepath"
	"testing"
)

func TestDataRootDerivedFromInstallRoot(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}

	got, err := DataRoot()
	if err != nil {
		t.Fatalf("DataRoot() error = %v", err)
	}

	if got != root {
		t.Fatalf("DataRoot() = %q, want %q", got, root)
	}
}

func TestCacheRootDerivedFromDataRoot(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}

	got, err := CacheRoot()
	if err != nil {
		t.Fatalf("CacheRoot() error = %v", err)
	}

	want := filepath.Join(root, ".cache")
	if got != want {
		t.Fatalf("CacheRoot() = %q, want %q", got, want)
	}
}

func TestUtilityPathUsesProgramRoot(t *testing.T) {
	programRoot, err := ProgramRoot()
	if err != nil {
		t.Fatalf("ProgramRoot() error = %v", err)
	}

	got, err := UtilityPath("sync.exe")
	if err != nil {
		t.Fatalf("UtilityPath(sync.exe) error = %v", err)
	}

	want := filepath.Join(programRoot, "utils", "sync.exe")
	if got != want {
		t.Fatalf("UtilityPath(sync.exe) = %q, want %q", got, want)
	}
}

func TestProgramSyncRootUsesProgramRoot(t *testing.T) {
	programRoot, err := ProgramRoot()
	if err != nil {
		t.Fatalf("ProgramRoot() error = %v", err)
	}

	got, err := ProgramSyncRoot()
	if err != nil {
		t.Fatalf("ProgramSyncRoot() error = %v", err)
	}

	want := filepath.Join(programRoot, ".sync")
	if got != want {
		t.Fatalf("ProgramSyncRoot() = %q, want %q", got, want)
	}
}

func TestDataSyncRootDerivedFromDataRoot(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}

	got, err := DataSyncRoot()
	if err != nil {
		t.Fatalf("DataSyncRoot() error = %v", err)
	}

	want := filepath.Join(root, ".sync")
	if got != want {
		t.Fatalf("DataSyncRoot() = %q, want %q", got, want)
	}
}
