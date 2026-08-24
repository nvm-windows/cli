package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestAppsUninstallCommandIncludesFromApps(t *testing.T) {
	got := appsUninstallCommand(`C:\Program Files\Author Software\nvm\nvm.exe`, "22.15.0")
	want := `"C:\Program Files\Author Software\nvm\nvm.exe" uninstall 22.15.0 --from-apps`
	if got != want {
		t.Fatalf("appsUninstallCommand() = %q, want %q", got, want)
	}
}

func TestRegisterNodeVersionWritesQuietUninstallString(t *testing.T) {
	root := t.TempDir()
	setTestSetting(t, "root", root)

	version := "22.15.0"
	installDir := filepath.Join(root, "v"+version)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	nodeExe := filepath.Join(installDir, "node.exe")
	if err := os.WriteFile(nodeExe, []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile node.exe: %v", err)
	}

	registerNodeVersion(version, installDir, "OpenJS Foundation")

	key, err := registry.OpenKey(registry.CURRENT_USER, registryKeyName(version), registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("OpenKey: %v", err)
	}
	defer key.Close()
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, registryKeyName(version))
	})

	uninstall, _, err := key.GetStringValue("UninstallString")
	if err != nil {
		t.Fatalf("UninstallString: %v", err)
	}
	quiet, _, err := key.GetStringValue("QuietUninstallString")
	if err != nil {
		t.Fatalf("QuietUninstallString: %v", err)
	}
	if uninstall != quiet {
		t.Fatalf("UninstallString (%q) != QuietUninstallString (%q)", uninstall, quiet)
	}
	if !strings.Contains(uninstall, "uninstall "+version+" --from-apps") {
		t.Fatalf("UninstallString = %q, want uninstall %s --from-apps", uninstall, version)
	}

	loc, _, err := key.GetStringValue("InstallLocation")
	if err != nil {
		t.Fatalf("InstallLocation: %v", err)
	}
	if filepath.Clean(loc) != filepath.Clean(installDir) {
		t.Fatalf("InstallLocation = %q, want %q", loc, installDir)
	}
}

func TestLookupARPInstallLocation(t *testing.T) {
	root := t.TempDir()
	version := "20.20.2"
	installDir := filepath.Join(root, "v"+version)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyName(version), registry.SET_VALUE)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	key.SetStringValue("DisplayVersion", version)
	key.SetStringValue("InstallLocation", installDir)
	key.SetStringValue("ManagedBy", "nvm-windows")
	key.Close()
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, registryKeyName(version))
	})

	got := lookupARPInstallLocation(version)
	if filepath.Clean(got) != filepath.Clean(installDir) {
		t.Fatalf("lookupARPInstallLocation(%q) = %q, want %q", version, got, installDir)
	}
}

func TestUninstallFromAppsUsesARPInstallLocation(t *testing.T) {
	root := t.TempDir()
	// Intentionally point settings root elsewhere so scan misses the install.
	setTestSetting(t, "root", filepath.Join(root, "other-root"))
	if err := os.MkdirAll(filepath.Join(root, "other-root"), 0o755); err != nil {
		t.Fatalf("MkdirAll other-root: %v", err)
	}

	version := "16.20.2"
	installDir := filepath.Join(root, "actual", "v"+version)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll installDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "node.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile node.exe: %v", err)
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyName(version), registry.SET_VALUE)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	key.SetStringValue("DisplayVersion", version)
	key.SetStringValue("InstallLocation", installDir)
	key.SetStringValue("ManagedBy", "nvm-windows")
	key.Close()
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, registryKeyName(version))
	})

	oldReshim := runReshim
	runReshim = func() error { return nil }
	t.Cleanup(func() { runReshim = oldReshim })

	if err := Uninstall(UninstallConfig{
		Versions: []string{version},
		FromApps: true,
	}); err != nil {
		t.Fatalf("Uninstall(FromApps) error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(installDir, "node.exe")); !os.IsNotExist(err) {
		t.Fatalf("node.exe still present after FromApps uninstall: %v", err)
	}
	if _, err := registry.OpenKey(registry.CURRENT_USER, registryKeyName(version), registry.QUERY_VALUE); err == nil {
		t.Fatal("ARP key still present after FromApps uninstall")
	}
}
