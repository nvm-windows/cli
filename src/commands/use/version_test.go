package use

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
)

const useCommandTestRegistryRoot = "HKCU/Software/NVMTest/use_command_test"

func TestMain(m *testing.M) {
	prefs.ROOT = useCommandTestRegistryRoot
	prefs.ROOTS = []string{prefs.ROOT}
	code := m.Run()
	exec.Command("reg", "delete", `HKCU\Software\NVMTest`, "/f").Run() //nolint:errcheck
	os.Exit(code)
}

func TestNotInstalledUseErrorIncludesAutoInstallHintInShimMode(t *testing.T) {
	err := notInstalledUseError("22.0.0", "shim", true)
	if err == nil {
		t.Fatal("notInstalledUseError() = nil, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "v22.0.0 is not installed") {
		t.Fatalf("error = %q, want not installed message", msg)
	}
	if !strings.Contains(msg, "nvm config set auto_install=true") {
		t.Fatalf("error = %q, want auto-install hint", msg)
	}
}

func TestNotInstalledUseErrorOmitsAutoInstallHintInLinkMode(t *testing.T) {
	err := notInstalledUseError("22.0.0", "link", true)
	if err == nil {
		t.Fatal("notInstalledUseError() = nil, want error")
	}
	if strings.Contains(err.Error(), "auto_install") {
		t.Fatalf("error = %q, want no auto-install hint in link mode", err.Error())
	}
}

func TestNotInstalledUseErrorOmitsAutoInstallHintWhenEnabled(t *testing.T) {
	err := notInstalledUseError("22.0.0", "shim", false)
	if err == nil {
		t.Fatal("notInstalledUseError() = nil, want error")
	}
	if strings.Contains(err.Error(), "auto_install") {
		t.Fatalf("error = %q, want no auto-install hint when auto-install enabled", err.Error())
	}
}

func TestGetStringSettingReturnsEmptyForMissingValue(t *testing.T) {
	if err := settings.Del("active_version"); err != nil {
		t.Fatalf("Del(active_version) error = %v", err)
	}

	got, err := getStringSetting("active_version")
	if err != nil {
		t.Fatalf("getStringSetting(active_version) error = %v", err)
	}
	if got != "" {
		t.Fatalf("getStringSetting(active_version) = %q, want empty string", got)
	}
}

func TestGetStringSettingReturnsStoredValue(t *testing.T) {
	if err := settings.Put("active_version", "22.0.0"); err != nil {
		t.Fatalf("Put(active_version) error = %v", err)
	}

	got, err := getStringSetting("active_version")
	if err != nil {
		t.Fatalf("getStringSetting(active_version) error = %v", err)
	}
	if got != "22.0.0" {
		t.Fatalf("getStringSetting(active_version) = %q, want %q", got, "22.0.0")
	}
}
