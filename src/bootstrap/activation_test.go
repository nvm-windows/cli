//go:build windows

package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateVersionActivationAcceptsTrustedDirectoryAndNode(t *testing.T) {
	stubActivationBlockedLog(t, nil)
	versionDir := filepath.Join(t.TempDir(), "v22.0.0")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	originalVerify := verifyActivationNode
	verifyActivationNode = func(path string) error {
		if path != filepath.Join(versionDir, "node.exe") {
			t.Fatalf("verify path = %q", path)
		}
		return nil
	}
	t.Cleanup(func() { verifyActivationNode = originalVerify })

	if err := ValidateVersionActivation(versionDir); err != nil {
		t.Fatalf("ValidateVersionActivation() error = %v", err)
	}
}

func TestValidateVersionActivationRejectsReparseDirectory(t *testing.T) {
	var loggedKind string
	stubActivationBlockedLog(t, func(_, _, failureKind, _ string) {
		loggedKind = failureKind
	})
	root := t.TempDir()
	target := filepath.Join(root, "real")
	versionDir := filepath.Join(root, "v22.0.0")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, versionDir); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}

	err := ValidateVersionActivation(versionDir)
	if err == nil || !strings.Contains(err.Error(), "NVM blocked link-mode activation") || !strings.Contains(err.Error(), "NVM4302") {
		t.Fatalf("ValidateVersionActivation() error = %v, want actionable NVM4302 rejection", err)
	}
	if loggedKind != "reparse_point" {
		t.Fatalf("logged failure kind = %q, want reparse_point", loggedKind)
	}
}

func TestValidateVersionActivationRejectsUntrustedNode(t *testing.T) {
	versionDir := filepath.Join(t.TempDir(), "v22.0.0")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	originalVerify := verifyActivationNode
	verifyActivationNode = func(string) error {
		return errors.New("authenticode signature verification failed")
	}
	t.Cleanup(func() { verifyActivationNode = originalVerify })

	var loggedKind, loggedDetail string
	stubActivationBlockedLog(t, func(_, _, failureKind, detail string) {
		loggedKind = failureKind
		loggedDetail = detail
	})

	err := ValidateVersionActivation(versionDir)
	if err == nil || !strings.Contains(err.Error(), "integrity could not be verified") || !strings.Contains(err.Error(), "nvm install 22.0.0 --force") || !strings.Contains(err.Error(), "NVM4302") {
		t.Fatalf("ValidateVersionActivation() error = %v, want actionable NVM4302 verification failure", err)
	}
	if loggedKind != "executable_trust_failed" || loggedDetail != "authenticode signature verification failed" {
		t.Fatalf("logged failure = (%q, %q)", loggedKind, loggedDetail)
	}
}

func stubActivationBlockedLog(t *testing.T, replacement func(string, string, string, string)) {
	t.Helper()
	original := logActivationBlocked
	if replacement == nil {
		replacement = func(string, string, string, string) {}
	}
	logActivationBlocked = replacement
	t.Cleanup(func() { logActivationBlocked = original })
}
