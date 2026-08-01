package clitest

import (
	"common/settings"
	"os"
	"path/filepath"
	"testing"

	"nvm/bootstrap"
)

// InstallUtilityStub copies the current test binary to {ProgramRoot}/utils/{name}.
func InstallUtilityStub(t *testing.T, name string) {
	t.Helper()

	currentExe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}

	utilityPath, err := bootstrap.UtilityPath(name)
	if err != nil {
		t.Fatalf("UtilityPath(%q) error = %v", name, err)
	}

	if err := os.MkdirAll(filepath.Dir(utilityPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(utilityPath), err)
	}

	originalData, err := os.ReadFile(utilityPath)
	hadOriginal := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%q) error = %v", utilityPath, err)
	}

	currentExeData, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", currentExe, err)
	}

	if err := os.WriteFile(utilityPath, currentExeData, 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", utilityPath, err)
	}

	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(utilityPath, originalData, 0o755)
			return
		}
		_ = os.Remove(utilityPath)
	})
}

// PrepareMutatingCommand runs profile bootstrap for use/on/off tests.
// Link mode avoids async reshim.exe (test binary stub stays locked on Windows).
func (s *Sandbox) PrepareMutatingCommand() {
	s.t.Helper()
	if err := s.WriteSetting("mode", "link"); err != nil {
		s.t.Fatalf("Put(mode) error = %v", err)
	}
	settings.Load(true)
	if err := ensureBootstrap(s); err != nil {
		s.t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}
}

// ExecuteBootstrapped runs commands that mutate shims/links (use, on, off).
func (s *Sandbox) ExecuteBootstrapped(args ...string) (stdout, stderr string, err error) {
	s.t.Helper()
	s.PrepareMutatingCommand()
	return s.ExecuteWithOptions(ExecuteOptions{Bootstrap: false}, args...)
}
