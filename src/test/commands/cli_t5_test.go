package commands_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvm/commands"
	"nvm/test/clitest"
)

func TestCLIReshimCommand(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.ExecuteWithSyncStub("reshim")
	if err != nil {
		t.Fatalf("Execute(reshim) error = %v stderr = %q stdout = %q", err, stderr, stdout)
	}
}

func TestCLIDoctorCommand(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.ExecuteWithSyncStub("doctor")
	if err != nil {
		t.Fatalf("Execute(doctor) error = %v stderr = %q", err, stderr)
	}
}

func TestCLIUpgradeBlockedByPolicy(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("disable_upgrade", true); err != nil {
		t.Fatalf("WriteSetting(disable_upgrade) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("upgrade")
	if err != nil {
		t.Fatalf("Execute(upgrade) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "blocked by computer policy") {
		t.Fatalf("Execute(upgrade) stdout = %q, want policy block message", stdout)
	}
}

func TestCLIUpgradeCheckWithSyncStub(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("disable_upgrade", false); err != nil {
		t.Fatalf("WriteSetting(disable_upgrade) error = %v", err)
	}

	_, stderr, err := sb.ExecuteWithSyncStub("upgrade", "--check")
	if err != nil {
		t.Fatalf("Execute(upgrade --check) error = %v stderr = %q", err, stderr)
	}
}

func TestCLIUpgradeProtocolDispatchesSync(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("disable_upgrade", false); err != nil {
		t.Fatalf("WriteSetting(disable_upgrade) error = %v", err)
	}

	record := filepath.Join(sb.TempRoot, "sync-args.txt")
	t.Setenv("NVM_RESHIM_TEST_SYNC_RECORD", record)
	clitest.InstallUtilityStub(t, "sync.exe")
	t.Setenv("NVM_RESHIM_TEST_SYNC_HELPER", "1")
	t.Setenv("NVM_RESHIM_TEST_SYNC_EXIT_CODE", "0")

	if err := commands.HandleUpgradeProtocolURI("nvm://upgrade?version=2.1.0-beta.4"); err != nil {
		t.Fatalf("HandleUpgradeProtocolURI error = %v", err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("ReadFile(record) error = %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "upgrade") || !strings.Contains(text, "--release") || !strings.Contains(text, "2.1.0-beta.4") || !strings.Contains(text, "--force") {
		t.Fatalf("sync args = %q, want upgrade --force --release 2.1.0-beta.4", text)
	}
}
