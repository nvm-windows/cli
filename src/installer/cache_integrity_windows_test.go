//go:build windows

package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/registry"
	"common/settings"
	"common/verifycache"
)

func setupDownloadCacheTestProfile(t *testing.T) string {
	t.Helper()

	dataRoot := t.TempDir()
	installRoot := filepath.Join(dataRoot, "installs")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(installRoot) error = %v", err)
	}

	key := `HKCU/Software/NVMTest/download_cache/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load(true)

	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	settings.Load(true)

	t.Cleanup(func() {
		_ = exec.Command("reg", "delete", `HKCU\Software\NVMTest\download_cache`, "/f").Run()
	})

	return dataRoot
}

func TestVerifyCachedNodeArchiveIntegrityEndToEndTPM(t *testing.T) {
	dataRoot := setupDownloadCacheTestProfile(t)
	cacheDir := filepath.Join(dataRoot, ".cache", "versions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}

	archivePath := filepath.Join(cacheDir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("verified archive bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := verifycache.SignDownloadArchiveCache(archivePath); err != nil {
		t.Fatalf("SignDownloadArchiveCache() error = %v", err)
	}

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityRejectsTamperedArchiveEndToEnd(t *testing.T) {
	dataRoot := setupDownloadCacheTestProfile(t)
	cacheDir := filepath.Join(dataRoot, ".cache", "versions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}

	archivePath := filepath.Join(cacheDir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("verified archive bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := verifycache.SignDownloadArchiveCache(archivePath); err != nil {
		t.Fatalf("SignDownloadArchiveCache() error = %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("tampered archive bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile(tampered) error = %v", err)
	}

	err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{})
	if err == nil {
		t.Fatal("verifyCachedNodeArchiveIntegrity() error = nil, want integrity failure")
	}
	if !strings.Contains(err.Error(), "integrity check") {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v, want integrity check failure", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityLegacyFallbackEndToEnd(t *testing.T) {
	dataRoot := setupDownloadCacheTestProfile(t)
	cacheDir := filepath.Join(dataRoot, ".cache", "versions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}

	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(cacheDir, archiveName)
	content := []byte("legacy cached archive without registry entry")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(ctx context.Context, version, path string, cfg InstallConfig) error {
		if path != archivePath {
			t.Fatalf("mirror verify path = %q, want %q", path, archivePath)
		}
		if version != "22.0.0" {
			t.Fatalf("mirror verify version = %q, want 22.0.0", version)
		}
		return nil
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}

	if err := verifycache.VerifyDownloadArchiveCache(archivePath); err != nil {
		t.Fatalf("VerifyDownloadArchiveCache() after fallback sign error = %v", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityLocalSHASUMEndToEnd(t *testing.T) {
	dataRoot := setupDownloadCacheTestProfile(t)
	cacheDir := filepath.Join(dataRoot, ".cache", "versions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}

	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(cacheDir, archiveName)
	content := []byte("offline local shasum archive")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	shasumPath := localSHASUMPath("22.0.0", archivePath)
	hash := sha256.Sum256(content)
	shasumLine := hex.EncodeToString(hash[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	mirrorCalled := false
	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error {
		mirrorCalled = true
		return errors.New("mirror should not be called")
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if mirrorCalled {
		t.Fatal("mirror SHASUM verification was called, want local SHASUM only")
	}
	if err := verifycache.VerifyDownloadArchiveCache(archivePath); err != nil {
		t.Fatalf("VerifyDownloadArchiveCache() after local SHASUM sign error = %v", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityTrustedLocalInstallEndToEnd(t *testing.T) {
	dataRoot := setupDownloadCacheTestProfile(t)
	setupMachinePolicyTrustedLocalInstall(t, dataRoot)

	cacheDir := filepath.Join(dataRoot, "media")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(cacheDir) error = %v", err)
	}

	archivePath := filepath.Join(cacheDir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("airgapped trusted archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	mirrorCalled := false
	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error {
		mirrorCalled = true
		return errors.New("mirror should not be called")
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if mirrorCalled {
		t.Fatal("mirror SHASUM verification was called, want trusted local bypass")
	}
	if err := verifycache.VerifyDownloadArchiveCache(archivePath); err != nil {
		t.Fatalf("VerifyDownloadArchiveCache() after trusted local sign error = %v", err)
	}
}

func setupMachinePolicyTrustedLocalInstall(t *testing.T, trustedDir string) {
	t.Helper()

	key := `HKCU/Software/NVMTest/machine_policy/` + strings.ReplaceAll(t.Name(), "/", "_")
	oldRoot := prefs.MACHINE_POLICY_ROOT
	prefs.MACHINE_POLICY_ROOT = key
	t.Cleanup(func() {
		prefs.MACHINE_POLICY_ROOT = oldRoot
		_ = exec.Command("reg", "delete", `HKCU\Software\NVMTest\machine_policy`, "/f").Run()
	})

	if err := registry.Put(trustedDir, key+"/LocalInstallDir"); err != nil {
		t.Fatalf("Put(LocalInstallDir) error = %v", err)
	}
	if err := registry.Put(uint32(1), key+"/LocalInstallOnly"); err != nil {
		t.Fatalf("Put(LocalInstallOnly) error = %v", err)
	}
}
