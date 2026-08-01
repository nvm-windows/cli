//go:build windows

package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

func setupVerificationTestProfile(t *testing.T) {
	t.Helper()

	key := `HKCU/Software/NVMTest/installer_verify/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load(true)

	t.Cleanup(func() {
		_ = exec.Command("reg", "delete", `HKCU\Software\NVMTest\installer_verify`, "/f").Run()
	})
}

func signedNodeExecutable(t *testing.T) string {
	t.Helper()

	if override := strings.TrimSpace(os.Getenv("NVM_TEST_SIGNED_NODE")); override != "" {
		if _, statErr := os.Stat(override); statErr == nil {
			return override
		}
		t.Fatalf("NVM_TEST_SIGNED_NODE=%q is not accessible", override)
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		t.Skip("LOCALAPPDATA is not set")
	}

	matches, err := filepath.Glob(filepath.Join(localAppData, "Author Software", "nvm", "installs", "*", "node.exe"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	for _, candidate := range matches {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	t.Skip("no signed node.exe found; set NVM_TEST_SIGNED_NODE to run this test")
	return ""
}

func TestVerifyAllowedSignerUsesWinVerifyTrust(t *testing.T) {
	dir := t.TempDir()
	unsignedPath := filepath.Join(dir, "unsigned.bin")
	if err := os.WriteFile(unsignedPath, []byte("not a signed executable"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := verifyAllowedSigner(unsignedPath)
	if err == nil {
		t.Fatal("verifyAllowedSigner() error = nil, want authenticode failure")
	}
	if !strings.Contains(err.Error(), "authenticode signature verification failed") {
		t.Fatalf("verifyAllowedSigner() error = %q", err.Error())
	}
}

func TestVerifyAllowedSignerRejectsUnsignedEvenWhenOrgAllowed(t *testing.T) {
	setupVerificationTestProfile(t)

	dir := t.TempDir()
	unsignedPath := filepath.Join(dir, "unsigned.bin")
	if err := os.WriteFile(unsignedPath, []byte("not a signed executable"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := settings.Put("allowed_signers", "OpenJS Foundation,Node.js Foundation"); err != nil {
		t.Fatalf("Put(allowed_signers) error = %v", err)
	}
	settings.Load(true)

	_, err := verifyAllowedSigner(unsignedPath)
	if err == nil {
		t.Fatal("verifyAllowedSigner() error = nil, want authenticode failure before org policy")
	}
	if !strings.Contains(err.Error(), "authenticode signature verification failed") {
		t.Fatalf("verifyAllowedSigner() error = %q", err.Error())
	}
}

func TestVerifyAllowedSignerAcceptsSignedNode(t *testing.T) {
	setupVerificationTestProfile(t)
	nodePath := signedNodeExecutable(t)

	settings.Load(true)

	publisher, err := verifyAllowedSigner(nodePath)
	if err != nil {
		t.Fatalf("verifyAllowedSigner(%q) error = %v", nodePath, err)
	}
	if strings.TrimSpace(publisher) == "" {
		t.Fatal("verifyAllowedSigner() publisher = empty, want signer organization")
	}
}

func TestVerifyAllowedSignerHonorsAllowedSignersPolicy(t *testing.T) {
	setupVerificationTestProfile(t)
	nodePath := signedNodeExecutable(t)

	if err := settings.Put("allowed_signers", "Microsoft Windows"); err != nil {
		t.Fatalf("Put(allowed_signers) error = %v", err)
	}
	settings.Load(true)

	_, err := verifyAllowedSigner(nodePath)
	if err == nil {
		t.Fatal("verifyAllowedSigner() error = nil, want disallowed signer error")
	}
	if !strings.Contains(err.Error(), "is not allowed") {
		t.Fatalf("verifyAllowedSigner() error = %q", err.Error())
	}
}
