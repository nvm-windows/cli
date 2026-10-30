package clitest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

// Sandbox is an isolated nvm profile for CLI tests.
type Sandbox struct {
	t           *testing.T
	TempRoot    string
	InstallRoot string
	RegistryKey string
}

// NewSandbox creates a temp data root and isolated HKCU registry namespace.
func NewSandbox(t *testing.T) *Sandbox {
	t.Helper()

	tempRoot := t.TempDir()
	// Mirror production: DataRoot = %LOCALAPPDATA%\Author Software\nvm, InstallRoot = DataRoot\installs.
	localAppData := filepath.Join(tempRoot, "Local")
	dataRoot := filepath.Join(localAppData, "Author Software", "nvm")
	installRoot := filepath.Join(dataRoot, "installs")
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(installRoot) error = %v", err)
	}
	t.Setenv("LOCALAPPDATA", localAppData)

	safeName := sanitizeTestName(t.Name())
	registryKey := fmt.Sprintf("HKCU/Software/NVMTest/clitest_%s", safeName)

	s := &Sandbox{
		t:           t,
		TempRoot:    dataRoot,
		InstallRoot: installRoot,
		RegistryKey: registryKey,
	}

	t.Cleanup(s.Cleanup)
	s.Apply()
	return s
}

// Apply binds preferences to the sandbox registry and seeds default settings.
func (s *Sandbox) Apply() {
	s.t.Helper()

	prefs.ROOT = s.RegistryKey
	prefs.ROOTS = []string{s.RegistryKey}

	s.deleteRegistryKey()

	settings.Load(true)
	if err := settings.Put("root", s.InstallRoot); err != nil {
		s.t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		s.t.Fatalf("Put(mode) error = %v", err)
	}
	settings.Load(true)
	ReloadCacheStore()
}

// Cleanup removes the sandbox registry key.
func (s *Sandbox) Cleanup() {
	s.deleteRegistryKey()
}

// DataRoot returns the per-user NVM home (parent of InstallRoot in the default layout).
func (s *Sandbox) DataRoot() string {
	return s.TempRoot
}

// WriteSetting persists a configuration value in the sandbox registry.
func (s *Sandbox) WriteSetting(name string, value any) error {
	return settings.Put(name, value)
}

// ReadSetting reads a configuration value from the sandbox registry.
func (s *Sandbox) ReadSetting(name string) (any, error) {
	return settings.Get(name)
}

func (s *Sandbox) deleteRegistryKey() {
	regPath := strings.ReplaceAll(strings.TrimPrefix(s.RegistryKey, "HKCU/"), "/", `\`)
	_ = exec.Command("reg", "delete", `HKCU\`+regPath, "/f").Run()
}

func sanitizeTestName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	safe := replacer.Replace(name)
	if len(safe) > 80 {
		safe = safe[len(safe)-80:]
	}
	return safe
}
