package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	prefs "common/preferences"
	"common/settings"

	"golang.org/x/sys/windows"
)

const installerTestRegistryRoot = "HKCU/Software/NVMTest/installer_test"

func TestMain(m *testing.M) {
	if exitCode, handled := maybeRunReshimTestHelper(); handled {
		os.Exit(exitCode)
	}

	prefs.ROOT = installerTestRegistryRoot
	prefs.ROOTS = []string{prefs.ROOT}
	code := m.Run()
	exec.Command("reg", "delete", `HKCU\Software\NVMTest`, "/f").Run() //nolint:errcheck
	os.Exit(code)
}

func TestScanInstalledVersionsReturnsDescendingSemverOrder(t *testing.T) {
	root := t.TempDir()
	setTestSetting(t, "root", root)

	createInstalledVersion(t, root, "9.10.0")
	createInstalledVersion(t, root, "10.0.0")
	createInstalledVersion(t, root, "22.1.0")

	installed := scanInstalledVersions()
	want := []string{"22.1.0", "10.0.0", "9.10.0"}
	if len(installed) != len(want) {
		t.Fatalf("scanInstalledVersions() len = %d, want %d (%v)", len(installed), len(want), installed)
	}
	for i := range want {
		if installed[i] != want[i] {
			t.Fatalf("scanInstalledVersions()[%d] = %q, want %q (full=%v)", i, installed[i], want[i], installed)
		}
	}
}

func TestPrepareActiveForUninstallChoosesLatestSemverFallback(t *testing.T) {
	root := t.TempDir()
	setTestSetting(t, "root", root)
	setTestSetting(t, "active_version", "22.1.0")
	setTestSetting(t, "last_version", "")

	createInstalledVersion(t, root, "9.10.0")
	createInstalledVersion(t, root, "10.0.0")
	createInstalledVersion(t, root, "22.1.0")

	if err := prepareActiveForUninstall(map[string]struct{}{"22.1.0": {}}, func(string) {}); err != nil {
		t.Fatalf("prepareActiveForUninstall() error = %v", err)
	}

	active, err := settings.Get("active_version")
	if err != nil {
		t.Fatalf("Get(active_version) error = %v", err)
	}
	if got := active.(string); got != "10.0.0" {
		t.Fatalf("active_version = %q, want %q", got, "10.0.0")
	}

	last, err := settings.Get("last_version")
	if err != nil {
		t.Fatalf("Get(last_version) error = %v", err)
	}
	if got := last.(string); got != "" {
		t.Fatalf("last_version = %q, want empty string", got)
	}
}

func setTestSetting(t *testing.T, name, value string) {
	t.Helper()
	if err := settings.Put(name, value); err != nil {
		t.Fatalf("Put(%q, %q) error = %v", name, value, err)
	}
	settings.Load(true)
}

func createInstalledVersion(t *testing.T, root, version string) {
	t.Helper()
	versionDir := filepath.Join(root, fmt.Sprintf("v%s", version))
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", versionDir, err)
	}
	nodePath := filepath.Join(versionDir, "node.exe")
	if err := os.WriteFile(nodePath, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", nodePath, err)
	}
}

func TestHealInstalledVersionVisibilityClearsHiddenVersionDirectory(t *testing.T) {
	root := t.TempDir()
	setTestSetting(t, "root", root)
	createInstalledVersion(t, root, "22.22.2")
	destination := filepath.Join(root, "v22.22.2")
	if err := setHiddenPath(destination); err != nil {
		t.Fatalf("setHiddenPath(%q) error = %v", destination, err)
	}

	healInstalledVersionVisibility(destination)

	hidden, err := isHiddenPath(destination)
	if err != nil {
		t.Fatalf("isHiddenPath(%q) error = %v", destination, err)
	}
	if hidden {
		t.Fatalf("destination %q should not be hidden", destination)
	}
}

func setHiddenPath(path string) error {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	return windows.SetFileAttributes(ptr, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
}

func isHiddenPath(path string) (bool, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attrs, err := windows.GetFileAttributes(ptr)
	if err != nil {
		return false, err
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0, nil
}
