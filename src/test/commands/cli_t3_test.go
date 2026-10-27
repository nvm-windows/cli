package commands_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	if err == nil {
		t.Fatal("Execute(install not-a-real-alias) expected error, got nil")
	}
	combined := strings.ToLower(stdout + stderr + err.Error())
	if !strings.Contains(combined, "unknown alias") && !strings.Contains(combined, "failed") {
		t.Fatalf("Execute(install not-a-real-alias) output = stdout:%q stderr:%q err:%v", stdout, stderr, err)
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
	if err := sb.WriteSetting("local_install_only", true); err != nil {
		t.Fatalf("WriteSetting(local_install_only) error = %v", err)
	}

	prevFixtures := os.Getenv("NVM_TEST_HTTP_FIXTURES")
	if err := os.Setenv("NVM_TEST_HTTP_FIXTURES", "1"); err != nil {
		t.Fatalf("Setenv(NVM_TEST_HTTP_FIXTURES) error = %v", err)
	}
	t.Cleanup(func() {
		if prevFixtures == "" {
			_ = os.Unsetenv("NVM_TEST_HTTP_FIXTURES")
			return
		}
		_ = os.Setenv("NVM_TEST_HTTP_FIXTURES", prevFixtures)
	})

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "catalog should not be fetched", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	if err := sb.WriteSetting("node_mirror", srv.URL); err != nil {
		t.Fatalf("WriteSetting(node_mirror) error = %v", err)
	}

	stdout, stderr, err := sb.ExecuteWithSyncStub("install", "22.0.0")
	if err == nil {
		t.Fatal("Execute(install 22.0.0) expected error, got nil")
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("catalog fetched %d time(s), want 0 for exact local-only install", n)
	}
	combined := stdout + stderr + err.Error()
	if !strings.Contains(combined, "local install directory") {
		t.Fatalf("Execute(install 22.0.0) output = stdout:%q stderr:%q err:%v, want local install error", stdout, stderr, err)
	}
}

func TestCLIPinRequiresVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.Execute("pin")
	if err == nil {
		t.Fatal("Execute(pin) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no version specified") {
		t.Fatalf("Execute(pin) error = %v stderr = %q, want missing version error", err, stderr)
	}
}

func TestCLIPinRejectsUnrecognizedFile(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	_, stderr, err := sb.Execute("pin", "--file", "custom.json")
	if err == nil {
		t.Fatal("Execute(pin --file custom.json) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "would not be recognized") {
		t.Fatalf("Execute(pin --file custom.json) error = %v stderr = %q", err, stderr)
	}
}

func TestCLIPinWritesNvmrc(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))
	sb.SeedVersion("22.0.0", nil)

	projectDir := t.TempDir()
	t.Chdir(projectDir)

	_, stderr, err := sb.Execute("pin", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(pin 22.0.0) error = %v stderr = %q", err, stderr)
	}

	data, readErr := os.ReadFile(filepath.Join(projectDir, ".nvmrc"))
	if readErr != nil {
		t.Fatalf("ReadFile(.nvmrc) error = %v", readErr)
	}
	if strings.TrimSpace(string(data)) != "22.0.0" {
		t.Fatalf(".nvmrc = %q, want 22.0.0", string(data))
	}
}
