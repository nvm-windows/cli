package installer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"nvm/bootstrap"
)

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

func installReshimTestSyncBinary(t *testing.T) string {
	t.Helper()

	currentExe, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}

	syncPath, err := bootstrap.UtilityPath("sync.exe")
	if err != nil {
		t.Fatalf("UtilityPath(sync.exe) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(syncPath), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(syncPath), err)
	}

	originalData, err := os.ReadFile(syncPath)
	hadOriginal := err == nil
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadFile(%q) error = %v", syncPath, err)
	}

	currentExeData, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", currentExe, err)
	}

	if err := os.WriteFile(syncPath, currentExeData, 0755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", syncPath, err)
	}

	t.Cleanup(func() {
		if hadOriginal {
			_ = os.WriteFile(syncPath, originalData, 0755)
			return
		}

		_ = os.Remove(syncPath)
	})

	return syncPath
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

func TestReshimInvokesSyncCommand(t *testing.T) {
	syncPath := installReshimTestSyncBinary(t)
	recordPath := filepath.Join(t.TempDir(), "sync-invocation.txt")
	t.Setenv(reshimTestHelperEnv, "1")
	t.Setenv(reshimTestRecordEnv, recordPath)
	t.Setenv(reshimTestExitCodeEnv, "0")

	if err := reshim(); err != nil {
		t.Fatalf("reshim() error = %v", err)
	}

	gotPath, gotArgs := readReshimTestInvocation(t, recordPath)
	if gotPath != syncPath {
		t.Fatalf("reshim() path = %q, want %q", gotPath, syncPath)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "reshim" || gotArgs[1] != "--silent" {
		t.Fatalf("reshim() args = %v, want [reshim --silent]", gotArgs)
	}
}

func TestReshimWrapsSyncCommandFailure(t *testing.T) {
	_ = installReshimTestSyncBinary(t)
	t.Setenv(reshimTestHelperEnv, "1")
	t.Setenv(reshimTestExitCodeEnv, "17")

	err := reshim()
	if err == nil {
		t.Fatal("reshim() error = nil, want wrapped sync failure")
	}
	if got := err.Error(); !strings.Contains(got, "failed to reshim:") || !strings.Contains(got, "exit status 17") {
		t.Fatalf("reshim() error = %q, want wrapped exit status 17", got)
	}
}
