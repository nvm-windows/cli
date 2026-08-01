package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvm/test/clitest"
)

func versionDir(sb *clitest.Sandbox, version string) string {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	return filepath.Join(sb.InstallRoot, "v"+version)
}

func TestCLIUninstallRemovesSeededVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)

	stdout, stderr, err := sb.ExecuteWithSyncStub("uninstall", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(uninstall 22.0.0) error = %v stderr = %q stdout = %q", err, stderr, stdout)
	}
	if _, statErr := os.Stat(versionDir(sb, "22.0.0")); !os.IsNotExist(statErr) {
		t.Fatalf("version dir still exists after uninstall: %v", statErr)
	}
}

func TestCLIInstallFailsUnknownVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))

	stdout, stderr, err := sb.ExecuteWithSyncStub("install", "not-a-real-alias")
	if err != nil {
		t.Fatalf("Execute(install not-a-real-alias) unexpected error = %v", err)
	}
	combined := strings.ToLower(stdout + stderr)
	if !strings.Contains(combined, "unknown alias") && !strings.Contains(combined, "failed") {
		t.Fatalf("Execute(install not-a-real-alias) output = stdout:%q stderr:%q", stdout, stderr)
	}
}

func TestCLIInstallSkipsAlreadyInstalled(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))
	sb.SeedVersion("22.0.0", nil)

	stdout, stderr, err := sb.ExecuteWithSyncStub("install", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(install 22.0.0) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout+stderr, "SKIPPED") {
		t.Fatalf("Execute(install 22.0.0) output = stdout:%q stderr:%q, want SKIPPED", stdout, stderr)
	}
}

func TestCLIInstallLocalOnlyMissingArchive(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))
	if err := sb.WriteSetting("local_install_only", true); err != nil {
		t.Fatalf("WriteSetting(local_install_only) error = %v", err)
	}

	stdout, stderr, err := sb.ExecuteWithSyncStub("install", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(install 22.0.0) unexpected error = %v stderr = %q", err, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "local install directory") {
		t.Fatalf("Execute(install 22.0.0) output = stdout:%q stderr:%q, want local install error", stdout, stderr)
	}
}

func TestCLIRCRequiresVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.Execute("rc")
	if err == nil {
		t.Fatal("Execute(rc) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no version specified") {
		t.Fatalf("Execute(rc) error = %v stderr = %q, want missing version error", err, stderr)
	}
}

func TestCLIRCRejectsUnrecognizedFile(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	_, stderr, err := sb.Execute("rc", "--file", "custom.json")
	if err == nil {
		t.Fatal("Execute(rc --file custom.json) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "would not be recognized") {
		t.Fatalf("Execute(rc --file custom.json) error = %v stderr = %q", err, stderr)
	}
}

func TestCLIRCWritesNvmrc(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))
	sb.SeedVersion("22.0.0", nil)

	projectDir := t.TempDir()
	t.Chdir(projectDir)

	_, stderr, err := sb.Execute("rc", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(rc 22.0.0) error = %v stderr = %q", err, stderr)
	}

	data, readErr := os.ReadFile(filepath.Join(projectDir, ".nvmrc"))
	if readErr != nil {
		t.Fatalf("ReadFile(.nvmrc) error = %v", readErr)
	}
	if strings.TrimSpace(string(data)) != "22.0.0" {
		t.Fatalf(".nvmrc = %q, want 22.0.0", string(data))
	}
}
