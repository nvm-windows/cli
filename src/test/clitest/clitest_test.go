package clitest

import (
	"strings"
	"testing"
)

func TestExecuteDefaultEmptyProfile(t *testing.T) {
	sb := NewSandbox(t)

	stdout, stderr, err := sb.Execute("default")
	if err != nil {
		t.Fatalf("Execute(default) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "Default") {
		t.Fatalf("Execute(default) stdout = %q, want Default line", stdout)
	}
}

func TestSeedVersionCreatesNodeExe(t *testing.T) {
	sb := NewSandbox(t)

	nodePath := sb.SeedVersion("22.0.0", nil)
	if !strings.HasSuffix(nodePath, `\v22.0.0\node.exe`) && !strings.HasSuffix(nodePath, `/v22.0.0/node.exe`) {
		t.Fatalf("SeedVersion() = %q, want .../v22.0.0/node.exe", nodePath)
	}
}

func TestWriteAndReadSetting(t *testing.T) {
	sb := NewSandbox(t)

	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	got, err := sb.ReadSetting("active_version")
	if err != nil {
		t.Fatalf("ReadSetting(active_version) error = %v", err)
	}
	if got != "22.0.0" {
		t.Fatalf("ReadSetting(active_version) = %v, want %q", got, "22.0.0")
	}
}

func TestExecuteDefaultShowsActiveVersion(t *testing.T) {
	sb := NewSandbox(t)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("default")
	if err != nil {
		t.Fatalf("Execute(default) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "v22.0.0") {
		t.Fatalf("Execute(default) stdout = %q, want active version", stdout)
	}
}
