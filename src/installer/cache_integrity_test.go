package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"common/verifycache"
)

func TestVerifyCachedNodeArchiveIntegrityUsesTPMEntry(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return nil }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityRejectsBadTPMEntry(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return errors.New("download cache signature invalid") }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{})
	if err == nil {
		t.Fatal("verifyCachedNodeArchiveIntegrity() error = nil, want integrity failure")
	}
}

func TestVerifyCachedNodeArchiveIntegrityUsesLocalSHASUM(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	content := []byte("offline cached archive")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	shasumPath := localSHASUMPath("22.0.0", archivePath)
	hash := sha256.Sum256(content)
	shasumLine := hex.EncodeToString(hash[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	mirrorCalled := false
	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error {
		mirrorCalled = true
		return errors.New("mirror should not be called")
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	signed := false
	origSign := signDownloadArchiveCacheFn
	signDownloadArchiveCacheFn = func(string) error {
		signed = true
		return nil
	}
	t.Cleanup(func() { signDownloadArchiveCacheFn = origSign })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if mirrorCalled {
		t.Fatal("mirror SHASUM verification was called, want local SHASUM only")
	}
	if !signed {
		t.Fatal("expected SignDownloadArchiveCache after local SHASUM verify")
	}
}

func TestVerifyCachedNodeArchiveIntegrityRejectsBadLocalSHASUM(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	if err := os.WriteFile(archivePath, []byte("offline cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	shasumPath := localSHASUMPath("22.0.0", archivePath)
	if err := os.WriteFile(shasumPath, []byte("deadbeef  "+archiveName+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{})
	if err == nil {
		t.Fatal("verifyCachedNodeArchiveIntegrity() error = nil, want local SHASUM failure")
	}
	if !strings.Contains(err.Error(), "local SHASUM verification") {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v, want local SHASUM failure", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityTrustedLocalInstallBypass(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("trusted local archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	origTrusted := isMachinePolicyTrustedLocalArchiveFn
	isMachinePolicyTrustedLocalArchiveFn = func(string) (bool, error) { return true, nil }
	t.Cleanup(func() { isMachinePolicyTrustedLocalArchiveFn = origTrusted })

	mirrorCalled := false
	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error {
		mirrorCalled = true
		return errors.New("mirror should not be called")
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	signed := false
	origSign := signDownloadArchiveCacheFn
	signDownloadArchiveCacheFn = func(string) error {
		signed = true
		return nil
	}
	t.Cleanup(func() { signDownloadArchiveCacheFn = origSign })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if mirrorCalled {
		t.Fatal("mirror SHASUM verification was called, want trusted local bypass")
	}
	if !signed {
		t.Fatal("expected SignDownloadArchiveCache after trusted local bypass")
	}
}

func TestVerifyCachedNodeArchiveIntegrityFallsBackToMirrorSHASUM(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	content := []byte("legacy cached archive")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	shasumPath := filepath.Join(dir, "SHASUMS256-v22.0.0-win-x64.txt")
	hash := sha256.Sum256(content)
	shasumLine := hex.EncodeToString(hash[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	origTrusted := isMachinePolicyTrustedLocalArchiveFn
	isMachinePolicyTrustedLocalArchiveFn = func(string) (bool, error) { return false, nil }
	t.Cleanup(func() { isMachinePolicyTrustedLocalArchiveFn = origTrusted })

	signed := false
	origSign := signDownloadArchiveCacheFn
	signDownloadArchiveCacheFn = func(string) error {
		signed = true
		return nil
	}
	t.Cleanup(func() { signDownloadArchiveCacheFn = origSign })

	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(_ context.Context, _ string, path string, _ InstallConfig) error {
		ok, err := verifyNodeSHASUM(path, shasumPath)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("SHASUM verification failed")
		}
		return nil
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if !signed {
		t.Fatal("expected SignDownloadArchiveCache after mirror SHASUM verify")
	}
}

func TestVerifyCachedNodeArchiveIntegrityFallbackRejectsBadSHASUM(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error {
		return errors.New("SHASUM verification failed")
	}
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{})
	if err == nil {
		t.Fatal("verifyCachedNodeArchiveIntegrity() error = nil, want SHASUM failure")
	}
	if !strings.Contains(err.Error(), "SHASUM") {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v, want SHASUM failure", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityFallbackSignFailure(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return verifycache.ErrDownloadCacheMiss }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	origMirror := verifyArchiveWithMirrorSHASUMFn
	verifyArchiveWithMirrorSHASUMFn = func(context.Context, string, string, InstallConfig) error { return nil }
	t.Cleanup(func() { verifyArchiveWithMirrorSHASUMFn = origMirror })

	origSign := signDownloadArchiveCacheFn
	signDownloadArchiveCacheFn = func(string) error { return errors.New("TPM unavailable") }
	t.Cleanup(func() { signDownloadArchiveCacheFn = origSign })

	err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{})
	if err == nil {
		t.Fatal("verifyCachedNodeArchiveIntegrity() error = nil, want sign failure")
	}
	if !strings.Contains(err.Error(), "unable to sign download cache") {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v, want sign failure", err)
	}
}

func TestVerifyCachedNodeArchiveIntegrityRemovesLegacySidecarOnTPMHit(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cached archive"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	sidecarPath := archivePath + legacyCacheDigestSuffix
	if err := os.WriteFile(sidecarPath, []byte("legacy digest\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(sidecar) error = %v", err)
	}

	origVerify := verifyDownloadArchiveCacheFn
	verifyDownloadArchiveCacheFn = func(string) error { return nil }
	t.Cleanup(func() { verifyDownloadArchiveCacheFn = origVerify })

	if err := verifyCachedNodeArchiveIntegrity(t.Context(), "22.0.0", archivePath, InstallConfig{}); err != nil {
		t.Fatalf("verifyCachedNodeArchiveIntegrity() error = %v", err)
	}
	if _, err := os.Stat(sidecarPath); !os.IsNotExist(err) {
		t.Fatalf("legacy sidecar still exists: err = %v", err)
	}
}

func TestInvalidateCachedNodeArchiveRemovesLegacySidecar(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("cache"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(archivePath+legacyCacheDigestSuffix, []byte("legacy\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(sidecar) error = %v", err)
	}

	invalidateCachedNodeArchive(archivePath)

	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive still exists: err = %v", err)
	}
	if _, err := os.Stat(archivePath + legacyCacheDigestSuffix); !os.IsNotExist(err) {
		t.Fatalf("legacy sidecar still exists: err = %v", err)
	}
}

func TestCopyVerifiedArchiveToCacheCallsSign(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.7z")
	cacheDir := filepath.Join(dir, "versions")
	cacheFile := filepath.Join(cacheDir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(sourcePath, []byte("cached-by-copy"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	signed := false
	origSign := signDownloadArchiveCacheFn
	signDownloadArchiveCacheFn = func(path string) error {
		if path != cacheFile {
			t.Fatalf("sign path = %q, want %q", path, cacheFile)
		}
		signed = true
		return nil
	}
	t.Cleanup(func() { signDownloadArchiveCacheFn = origSign })

	if err := copyVerifiedArchiveToCache(sourcePath, cacheFile); err != nil {
		t.Fatalf("copyVerifiedArchiveToCache() error = %v", err)
	}
	if !signed {
		t.Fatal("copyVerifiedArchiveToCache() did not sign download cache")
	}
}
