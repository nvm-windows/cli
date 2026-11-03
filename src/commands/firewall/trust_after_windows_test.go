//go:build windows

package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

// Regression NVM4306: firewall trust must resign the JS entry proxy launches
// (pnpm.mjs), not only the top-level pnpm.cmd wrapper.
func TestFindModuleLaunchersIncludesPnpmMjs(t *testing.T) {
	dataRoot := t.TempDir()
	installRoot := filepath.Join(dataRoot, "installs")
	versionDir := filepath.Join(installRoot, "v22.0.0")
	pnpmBin := filepath.Join(versionDir, "node_modules", "pnpm", "bin")
	if err := os.MkdirAll(pnpmBin, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cmdPath := filepath.Join(versionDir, "pnpm.cmd")
	mjsPath := filepath.Join(pnpmBin, "pnpm.mjs")
	if err := os.WriteFile(cmdPath, []byte("@ECHO off\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(cmd): %v", err)
	}
	if err := os.WriteFile(mjsPath, []byte("export {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(mjs): %v", err)
	}

	key := `HKCU/Software/NVMTest/firewall/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load(true)
	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root): %v", err)
	}
	settings.Load(true)

	got := findModuleLaunchers("pnpm")
	hasCmd, hasMjs := false, false
	for _, p := range got {
		switch strings.ToLower(filepath.Clean(p)) {
		case strings.ToLower(filepath.Clean(cmdPath)):
			hasCmd = true
		case strings.ToLower(filepath.Clean(mjsPath)):
			hasMjs = true
		}
	}
	if !hasCmd {
		t.Fatalf("findModuleLaunchers(pnpm) missing %s; got %#v", cmdPath, got)
	}
	if !hasMjs {
		t.Fatalf("findModuleLaunchers(pnpm) missing %s (NVM4306 regression); got %#v", mjsPath, got)
	}
}

func TestFindModuleLaunchersIncludesPnpmHomeMjs(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cmdPath := filepath.Join(bin, "pnpm.cmd")
	mjsPath := filepath.Join(bin, "pnpm.mjs")
	if err := os.WriteFile(cmdPath, []byte("@ECHO off\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(cmd): %v", err)
	}
	if err := os.WriteFile(mjsPath, []byte("export {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(mjs): %v", err)
	}

	t.Setenv("PNPM_HOME", home)

	// Empty installs root so we only exercise PNPM_HOME discovery.
	key := `HKCU/Software/NVMTest/firewall/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load(true)
	emptyRoot := filepath.Join(t.TempDir(), "installs")
	if err := os.MkdirAll(emptyRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(emptyRoot): %v", err)
	}
	if err := settings.Put("root", emptyRoot); err != nil {
		t.Fatalf("Put(root): %v", err)
	}
	settings.Load(true)

	got := findModuleLaunchers("pnpm")
	hasMjs := false
	for _, p := range got {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(mjsPath)) {
			hasMjs = true
			break
		}
	}
	if !hasMjs {
		t.Fatalf("findModuleLaunchers(pnpm) missing PNPM_HOME mjs %s; got %#v", mjsPath, got)
	}
}
