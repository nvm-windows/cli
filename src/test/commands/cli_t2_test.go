package commands_test

import (
	"strings"
	"testing"

	"nvm/test/clitest"
)

func readActiveVersion(t *testing.T, sb *clitest.Sandbox) string {
	t.Helper()

	raw, err := sb.ReadSetting("active_version")
	if err != nil {
		t.Fatalf("ReadSetting(active_version) error = %v", err)
	}
	text, _ := raw.(string)
	return text
}

func readEnabled(t *testing.T, sb *clitest.Sandbox) bool {
	t.Helper()

	raw, err := sb.ReadSetting("enabled")
	if err != nil {
		t.Fatalf("ReadSetting(enabled) error = %v", err)
	}
	enabled, _ := raw.(bool)
	return enabled
}

func TestCLIUseFailsWhenNotInstalled(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.ExecuteBootstrapped("use", "99.99.99")
	if err == nil {
		t.Fatal("Execute(use 99.99.99) expected error, got nil")
	}
	combined := strings.ToLower(err.Error() + stderr)
	if !strings.Contains(combined, "not installed") && !strings.Contains(combined, "not found") {
		t.Fatalf("Execute(use 99.99.99) error = %v stderr = %q, want not installed", err, stderr)
	}
}

func TestCLIUseSwitchesActiveVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)

	stdout, stderr, err := sb.ExecuteBootstrapped("use", "22.0.0")
	if err != nil {
		t.Fatalf("Execute(use 22.0.0) error = %v stderr = %q", err, stderr)
	}
	if got := readActiveVersion(t, sb); got != "22.0.0" {
		t.Fatalf("active_version = %q, want 22.0.0", got)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(use 22.0.0) stdout = %q, want success message", stdout)
	}
}

func TestCLIUseLocalMatchesInstalledPartial(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	sb.SeedVersion("22.1.0", nil)

	_, stderr, err := sb.ExecuteBootstrapped("use", "22", "--local")
	if err != nil {
		t.Fatalf("Execute(use 22 --local) error = %v stderr = %q", err, stderr)
	}
	if got := readActiveVersion(t, sb); got != "22.1.0" {
		t.Fatalf("active_version = %q, want 22.1.0", got)
	}
}

func TestCLIUseLastRestoresPreviousVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("20.0.0", nil)
	sb.SeedVersion("22.0.0", nil)

	if _, _, err := sb.ExecuteBootstrapped("use", "22.0.0"); err != nil {
		t.Fatalf("Execute(use 22.0.0) error = %v", err)
	}
	if _, _, err := sb.ExecuteBootstrapped("use", "20.0.0"); err != nil {
		t.Fatalf("Execute(use 20.0.0) error = %v", err)
	}

	stdout, stderr, err := sb.ExecuteBootstrapped("use", "last")
	if err != nil {
		t.Fatalf("Execute(use last) error = %v stderr = %q", err, stderr)
	}
	if got := readActiveVersion(t, sb); got != "22.0.0" {
		t.Fatalf("active_version = %q, want 22.0.0 after use last", got)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(use last) stdout = %q, want success message", stdout)
	}
}

func TestCLIUseLastEmpty(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.ExecuteBootstrapped("use", "last")
	if err != nil {
		t.Fatalf("Execute(use last) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stderr, "No previously active version found") {
		t.Fatalf("Execute(use last) stderr = %q, want empty last message", stderr)
	}
}

func TestCLIAliasListEmpty(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("alias", "list")
	if err != nil {
		t.Fatalf("Execute(alias list) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "No aliases available") {
		t.Fatalf("Execute(alias list) stdout = %q, want empty message", stdout)
	}
}

func TestCLIAliasListSeeded(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("aliases", "work=22.0.0,stable=20.0.0"); err != nil {
		t.Fatalf("WriteSetting(aliases) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("alias", "list")
	if err != nil {
		t.Fatalf("Execute(alias list) error = %v stderr = %q", err, stderr)
	}
	for _, want := range []string{"work", "22.0.0", "stable", "20.0.0"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("Execute(alias list) stdout = %q, want %q", stdout, want)
		}
	}
}

func TestCLIAliasRemove(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("aliases", "work=22.0.0,stable=20.0.0"); err != nil {
		t.Fatalf("WriteSetting(aliases) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("alias", "remove", "work")
	if err != nil {
		t.Fatalf("Execute(alias remove work) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "1 alias removed") {
		t.Fatalf("Execute(alias remove work) stdout = %q, want removal message", stdout)
	}

	listOut, listErr, err := sb.Execute("alias", "list")
	if err != nil {
		t.Fatalf("Execute(alias list) error = %v stderr = %q", listErr, listErr)
	}
	if strings.Contains(listOut, "work") {
		t.Fatalf("Execute(alias list) stdout = %q, work alias should be gone", listOut)
	}
	if !strings.Contains(listOut, "stable") {
		t.Fatalf("Execute(alias list) stdout = %q, want stable alias", listOut)
	}
}

func TestCLIAliasAddRejectsSpaces(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.Execute("alias", "add", "bad name", "22.0.0")
	if err == nil {
		t.Fatal("Execute(alias add bad name) expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot contain spaces") {
		t.Fatalf("Execute(alias add bad name) error = %v stderr = %q, want spaces error", err, stderr)
	}
}

func TestCLIToggleOffOn(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("enabled", true); err != nil {
		t.Fatalf("WriteSetting(enabled) error = %v", err)
	}

	if _, _, err := sb.ExecuteBootstrapped("use", "22.0.0"); err != nil {
		t.Fatalf("Execute(use 22.0.0) error = %v", err)
	}

	stdout, stderr, err := sb.ExecuteBootstrapped("off")
	if err != nil {
		t.Fatalf("Execute(off) error = %v stderr = %q", err, stderr)
	}
	if readEnabled(t, sb) {
		t.Fatal("enabled = true after off, want false")
	}
	if !strings.Contains(stdout, "no longer managing") {
		t.Fatalf("Execute(off) stdout = %q, want disabled message", stdout)
	}

	stdout, stderr, err = sb.ExecuteBootstrapped("on")
	if err != nil {
		t.Fatalf("Execute(on) error = %v stderr = %q", err, stderr)
	}
	if !readEnabled(t, sb) {
		t.Fatal("enabled = false after on, want true")
	}
	if !strings.Contains(stdout, "now managing") {
		t.Fatalf("Execute(on) stdout = %q, want enabled message", stdout)
	}
}
