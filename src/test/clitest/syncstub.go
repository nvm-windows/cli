package clitest

import (
	"os"
	"strconv"
	"strings"
)

const (
	syncTestHelperEnv    = "NVM_RESHIM_TEST_SYNC_HELPER"
	syncTestRecordEnv    = "NVM_RESHIM_TEST_SYNC_RECORD"
	syncTestExitCodeEnv  = "NVM_RESHIM_TEST_SYNC_EXIT_CODE"
	reshimTestHelperEnv  = "NVM_RESHIM_TEST_RESHIM_HELPER"
	reshimTestRecordEnv  = "NVM_RESHIM_TEST_RESHIM_RECORD"
	reshimTestExitEnv    = "NVM_RESHIM_TEST_RESHIM_EXIT_CODE"
)

// RunReshimTestHelperIfRequested allows the test binary to act as a reshim.exe stub.
func RunReshimTestHelperIfRequested() (exitCode int, handled bool) {
	if os.Getenv(reshimTestHelperEnv) != "1" {
		return 0, false
	}

	if recordPath := strings.TrimSpace(os.Getenv(reshimTestRecordEnv)); recordPath != "" {
		lines := append([]string{strings.TrimSpace(os.Args[0])}, os.Args[1:]...)
		_ = os.WriteFile(recordPath, []byte(strings.Join(lines, "\n")), 0o644)
	}

	if rawExitCode := strings.TrimSpace(os.Getenv(reshimTestExitEnv)); rawExitCode != "" {
		exitCode, err := strconv.Atoi(rawExitCode)
		if err == nil {
			return exitCode, true
		}
	}

	return 0, true
}

// RunSyncTestHelperIfRequested allows the test binary to act as a sync.exe stub.
func RunSyncTestHelperIfRequested() (exitCode int, handled bool) {
	if os.Getenv(syncTestHelperEnv) != "1" {
		return 0, false
	}

	if recordPath := strings.TrimSpace(os.Getenv(syncTestRecordEnv)); recordPath != "" {
		lines := append([]string{strings.TrimSpace(os.Args[0])}, os.Args[1:]...)
		_ = os.WriteFile(recordPath, []byte(strings.Join(lines, "\n")), 0o644)
	}

	if rawExitCode := strings.TrimSpace(os.Getenv(syncTestExitCodeEnv)); rawExitCode != "" {
		exitCode, err := strconv.Atoi(rawExitCode)
		if err == nil {
			return exitCode, true
		}
	}

	return 0, true
}

// ExecuteWithSyncStub runs commands that invoke sync.exe with the test-binary stub.
func (s *Sandbox) ExecuteWithSyncStub(args ...string) (stdout, stderr string, err error) {
	s.t.Helper()
	InstallUtilityStub(s.t, "sync.exe")
	InstallUtilityStub(s.t, "reshim.exe")
	s.t.Setenv(syncTestHelperEnv, "1")
	s.t.Setenv(syncTestExitCodeEnv, "0")
	s.t.Setenv(reshimTestHelperEnv, "1")
	s.t.Setenv(reshimTestExitEnv, "0")
	return s.Execute(args...)
}
