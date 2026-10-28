package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyNodeSHASUMMatchesArchive(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	content := []byte("nvm clitest archive payload")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	sum := sha256.Sum256(content)
	shasumPath := filepath.Join(dir, "SHASUMS256.txt")
	shasumLine := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	ok, err := verifyNodeSHASUM(archivePath, shasumPath)
	if err != nil {
		t.Fatalf("verifyNodeSHASUM() error = %v", err)
	}
	if !ok {
		t.Fatal("verifyNodeSHASUM() = false, want true")
	}
}

func TestVerifyNodeSHASUMAcceptsStarPrefixedHash(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	content := []byte("star-prefix regression")
	if err := os.WriteFile(archivePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	sum := sha256.Sum256(content)
	shasumPath := filepath.Join(dir, "SHASUMS256.txt")
	shasumLine := "*" + hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	ok, err := verifyNodeSHASUM(archivePath, shasumPath)
	if err != nil {
		t.Fatalf("verifyNodeSHASUM() error = %v", err)
	}
	if !ok {
		t.Fatal("verifyNodeSHASUM() = false, want true for star-prefixed hash")
	}
}

func TestVerifyNodeSHASUMRejectsTamperedArchive(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	if err := os.WriteFile(archivePath, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	original := sha256.Sum256([]byte("original"))
	shasumPath := filepath.Join(dir, "SHASUMS256.txt")
	shasumLine := hex.EncodeToString(original[:]) + "  " + archiveName + "\n"
	if err := os.WriteFile(shasumPath, []byte(shasumLine), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	ok, err := verifyNodeSHASUM(archivePath, shasumPath)
	if err != nil {
		t.Fatalf("verifyNodeSHASUM() error = %v", err)
	}
	if ok {
		t.Fatal("verifyNodeSHASUM() = true, want false for tampered archive")
	}
}

func TestVerifyNodeSHASUMMissingEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "node-v22.0.0-win-x64.7z")
	if err := os.WriteFile(archivePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	shasumPath := filepath.Join(dir, "SHASUMS256.txt")
	if err := os.WriteFile(shasumPath, []byte("deadbeef  other.7z\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(shasum) error = %v", err)
	}

	ok, err := verifyNodeSHASUM(archivePath, shasumPath)
	if err == nil {
		t.Fatal("verifyNodeSHASUM() error = nil, want missing entry error")
	}
	if ok {
		t.Fatal("verifyNodeSHASUM() = true, want false")
	}
	if !strings.Contains(err.Error(), "SHASUM entry not found") {
		t.Fatalf("verifyNodeSHASUM() error = %q", err.Error())
	}
}

func TestVerifyNodeSHASUMMissingInputs(t *testing.T) {
	if _, err := verifyNodeSHASUM("", "shasum.txt"); err == nil {
		t.Fatal("verifyNodeSHASUM(empty file) error = nil, want error")
	}
	if _, err := verifyNodeSHASUM("archive.7z", ""); err == nil {
		t.Fatal("verifyNodeSHASUM(empty shasum) error = nil, want error")
	}
}

func TestVerifyNodeSHASUMFixtureRegression(t *testing.T) {
	dir := t.TempDir()
	archiveName := "node-v22.0.0-win-x64.7z"
	archivePath := filepath.Join(dir, archiveName)
	if err := os.WriteFile(archivePath, []byte("123"), 0o644); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	fixturePath := filepath.Join("..", "test", "testdata", "shasums.txt")
	ok, err := verifyNodeSHASUM(archivePath, fixturePath)
	if err != nil {
		t.Fatalf("verifyNodeSHASUM(fixture) error = %v", err)
	}
	if !ok {
		t.Fatal("verifyNodeSHASUM(fixture) = false, want true")
	}
}

func TestParseCoreVersion(t *testing.T) {
	got, ok := parseCoreVersion("v24.0.0")
	if !ok || got != (peCoreVersion{major: 24, minor: 0, patch: 0}) {
		t.Fatalf("parseCoreVersion(v24.0.0) = %#v ok=%v", got, ok)
	}
	got, ok = parseCoreVersion("24.0.0-nightly20240101")
	if !ok || got != (peCoreVersion{major: 24, minor: 0, patch: 0}) {
		t.Fatalf("parseCoreVersion(prerelease) = %#v ok=%v", got, ok)
	}
	if _, ok = parseCoreVersion(""); ok {
		t.Fatal("parseCoreVersion(empty) ok, want false")
	}
}

func TestCoreVersionEqualCatchesSwap(t *testing.T) {
	fileV10 := peCoreVersion{major: 10, minor: 0, patch: 0}
	if coreVersionEqual(fileV10, "24.0.0") {
		t.Fatal("Node 10 FileVersion must not match folder 24.0.0")
	}
	if !coreVersionEqual(fileV10, "10.0.0") {
		t.Fatal("Node 10 FileVersion must match folder 10.0.0")
	}
	if !coreVersionEqual(peCoreVersion{major: 24, minor: 1, patch: 0}, "24.1.0-rc.1") {
		t.Fatal("prerelease folder must compare on major.minor.patch")
	}
}

func TestVerifyNodeBinaryMatchesVersionSkipsMissingExe(t *testing.T) {
	dir := t.TempDir()
	if err := verifyNodeBinaryMatchesVersion(dir, "24.0.0"); err != nil {
		t.Fatalf("missing node.exe should skip: %v", err)
	}
}

func TestVerifyNodeBinaryMatchesVersionRejectsNonPE(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "node.exe"), []byte("not a pe"), 0o644); err != nil {
		t.Fatalf("WriteFile(node.exe) error = %v", err)
	}
	err := verifyNodeBinaryMatchesVersion(dir, "24.0.0")
	if err == nil || !strings.Contains(err.Error(), "NVM4314") {
		t.Fatalf("non-PE node.exe error = %v, want NVM4314", err)
	}
}
