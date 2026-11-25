package list

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

func TestGetInstalledVersionsMapRequiresNodeExe(t *testing.T) {
	root := t.TempDir()
	key := `HKCU/Software/NVMTest/list_installed_map/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load()
	t.Cleanup(func() {
		_ = exec.Command("reg", "delete", `HKCU\Software\NVMTest\list_installed_map`, "/f").Run()
	})

	complete := filepath.Join(root, "v24.22.3")
	hollow := filepath.Join(root, "v22.0.0")
	if err := os.MkdirAll(complete, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hollow, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(complete, "node.exe"), []byte("node"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := settings.Put("root", root); err != nil {
		t.Fatalf("settings.Put(root): %v", err)
	}

	got := getInstalledVersionsMap()
	if !got["v24.22.3"] {
		t.Fatalf("expected v24.22.3 installed, got %#v", got)
	}
	if got["v22.0.0"] {
		t.Fatalf("hollow v22.0.0 must not be marked installed, got %#v", got)
	}
}
