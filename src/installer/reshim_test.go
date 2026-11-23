package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"
	"nvm/bootstrap"
)

func setupReshimTestProfile(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	key := `HKCU/Software/NVMTest/installer_reshim/` + strings.ReplaceAll(t.Name(), "/", "_")
	prefs.ROOT = key
	prefs.ROOTS = []string{key}
	settings.Load()

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".shim"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.shim) error = %v", err)
	}

	t.Cleanup(func() {
		_ = exec.Command("reg", "delete", `HKCU\Software\NVMTest\installer_reshim`, "/f").Run()
	})
}

const (
	reshimTestHelperEnv   = "NVM_RESHIM_TEST_SYNC_HELPER"
	reshimTestRecordEnv   = "NVM_RESHIM_TEST_SYNC_RECORD"
	reshimTestExitCodeEnv = "NVM_RESHIM_TEST_SYNC_EXIT_CODE"
)

func maybeRunReshimTestHelper() (int, bool) {
	if os.Getenv(reshimTestHelperEnv) != "1" {
		return 0, false
	}

	if recordPath := strings.TrimSpace(os.Getenv(reshimTestRecordEnv)); recordPath != "" {
		lines := append([]string{filepath.Clean(os.Args[0])}, os.Args[1:]...)
		_ = os.WriteFile(recordPath, []byte(strings.Join(lines, "\n")), 0644)
	}

	if rawExitCode := strings.TrimSpace(os.Getenv(reshimTestExitCodeEnv)); rawExitCode != "" {
		exitCode, err := strconv.Atoi(rawExitCode)
		if err == nil {
			return exitCode, true
		}
	}

	return 0, true
}

func installReshimTestBinary(t *testing.T) string {
	t.Helper()

	currentExe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}

	reshimPath, err := bootstrap.UtilityPath("reshim.exe")
	if err != nil {
		t.Fatalf("UtilityPath(reshim.exe) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(reshimPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(reshimPath), err)
	}

	originalData, err := os.ReadFile(reshimPath)
	hadOriginal := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%q) error = %v", reshimPath, err)
	}

	currentExeData, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", currentExe, err)
	}

	if err := os.WriteFile(reshimPath, currentExeData, 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", reshimPath, err)
	}

	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(reshimPath, originalData, 0o755)
			return
		}

		_ = os.Remove(reshimPath)
	})

	return reshimPath
}

func readReshimTestInvocation(t *testing.T, recordPath string) (string, []string) {
	t.Helper()

	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", recordPath, err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("invocation record %q was empty", recordPath)
	}

	return filepath.Clean(lines[0]), lines[1:]
}

func TestReshimInvokesReshimUtility(t *testing.T) {
	setupReshimTestProfile(t)
	reshimPath := installReshimTestBinary(t)
	recordPath := filepath.Join(t.TempDir(), "reshim-invocation.txt")
	t.Setenv(reshimTestHelperEnv, "1")
	t.Setenv(reshimTestRecordEnv, recordPath)
	t.Setenv(reshimTestExitCodeEnv, "0")

	if err := reshim(); err != nil {
		t.Fatalf("reshim() error = %v", err)
	}

	gotPath, gotArgs := readReshimTestInvocation(t, recordPath)
	if gotPath != reshimPath {
		t.Fatalf("reshim() path = %q, want %q", gotPath, reshimPath)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "--silent" {
		t.Fatalf("reshim() args = %v, want [--silent]", gotArgs)
	}
}

func TestReshimWrapsUtilityFailure(t *testing.T) {
	setupReshimTestProfile(t)
	_ = installReshimTestBinary(t)
	t.Setenv(reshimTestHelperEnv, "1")
	t.Setenv(reshimTestExitCodeEnv, "17")

	err := reshim()
	if err == nil {
		t.Fatal("reshim() error = nil, want wrapped reshim failure")
	}
	if got := err.Error(); !strings.Contains(got, "reshim failed:") || !strings.Contains(got, "exit status 17") {
		t.Fatalf("reshim() error = %q, want wrapped exit status 17", got)
	}
}

func TestFinalizeInstallShimsRollsBackOnReshimFailure(t *testing.T) {
	setupReshimTestProfile(t)

	installDir := filepath.Join(t.TempDir(), "v22.0.0")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", installDir, err)
	}
	marker := filepath.Join(installDir, "node.exe")
	if err := os.WriteFile(marker, []byte("node"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", marker, err)
	}

	orig := runReshim
	t.Cleanup(func() { runReshim = orig })
	runReshim = func() error {
		return fmt.Errorf("reshim failed: exit status 1")
	}

	status := newStatus()
	status.TotalInstalled = 1
	status.Versions = []string{"22.0.0"}
	status.running = true

	txn := &Transaction{
		version:      "22.0.0",
		installDir:   installDir,
		installedNew: true,
		installed:    false,
	}

	err := finalizeInstallShims([]*Transaction{txn}, status, false, true)
	if err == nil {
		t.Fatal("finalizeInstallShims error = nil, want reshim failure")
	}
	if got := err.Error(); !strings.Contains(got, "reshim failed") {
		t.Fatalf("finalizeInstallShims error = %q, want reshim failed", got)
	}
	if _, statErr := os.Stat(installDir); !os.IsNotExist(statErr) {
		t.Fatalf("install dir %q still exists after reshim failure rollback", installDir)
	}
	if status.cancelMessage == "" {
		t.Fatal("status.cancelMessage empty, want rollback cancel message")
	}
}
