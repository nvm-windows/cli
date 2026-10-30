package bootstrap

import (
	"common/settings"
	"path/filepath"
	"testing"
)

func TestDataRootUsesLocalAppDataNotInstallParent(t *testing.T) {
	resetBootstrapState(t)

	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	oldOverride := programRootOverride
	programRootOverride = ""
	t.Cleanup(func() { programRootOverride = oldOverride })

	customInstall := filepath.Join(t.TempDir(), "D", "nvm")
	if err := settings.Put("root", customInstall); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}

	got, err := DataRoot()
	if err != nil {
		t.Fatalf("DataRoot() error = %v", err)
	}

	want := filepath.Join(local, "Author Software", "nvm")
	if got != want {
		t.Fatalf("DataRoot() = %q, want %q (must not be parent of InstallRoot)", got, want)
	}
}

func TestCacheRootDerivedFromDataRoot(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	oldOverride := programRootOverride
	programRootOverride = root
	t.Cleanup(func() { programRootOverride = oldOverride })

	if err := settings.Put("root", filepath.Join(t.TempDir(), "custom-installs")); err != nil {
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

	oldOverride := programRootOverride
	programRootOverride = root
	t.Cleanup(func() { programRootOverride = oldOverride })

	if err := settings.Put("root", filepath.Join(t.TempDir(), "custom-installs")); err != nil {
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
