package use

import (
	"os"
	"os/exec"
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
