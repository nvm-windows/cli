package commands_test

import (
	"strings"
	"testing"

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
