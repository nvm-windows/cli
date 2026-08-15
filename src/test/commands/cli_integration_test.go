package commands_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"nvm/test/clitest"
)

func TestCLIDefaultEmptyProfile(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("default")
	if err != nil {
		t.Fatalf("Execute(default) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "Default") {
		t.Fatalf("Execute(default) stdout = %q, want Default line", stdout)
	}
}

func TestCLIDefaultShowsActiveVersion(t *testing.T) {
	sb := clitest.NewSandbox(t)
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

func TestCLIDefaultJSON(t *testing.T) {
	sb := clitest.NewSandbox(t)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("default", "--json")
	if err != nil {
		t.Fatalf("Execute(default --json) error = %v stderr = %q", err, stderr)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v stdout = %q", err, stdout)
	}
	if out["default"] != "22.0.0" {
		t.Fatalf("default JSON = %q, want %q", out["default"], "22.0.0")
	}
}

func TestCLICurrentShowsDeprecation(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("current")
	if err != nil {
		t.Fatalf("Execute(current) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "Default") {
		t.Fatalf("Execute(current) stdout = %q, want Default line", stdout)
	}
	if !strings.Contains(stdout, "default") {
		t.Fatalf("Execute(current) stdout = %q, want deprecation notice", stdout)
	}
}

func TestCLIEnvText(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("env")
	if err != nil {
		t.Fatalf("Execute(env) error = %v stderr = %q", err, stderr)
	}

	installRoot := filepath.ToSlash(sb.InstallRoot)
	if !strings.Contains(stdout, installRoot) && !strings.Contains(stdout, strings.ReplaceAll(installRoot, "/", `\`)) {
		t.Fatalf("Execute(env) stdout missing install root %q", sb.InstallRoot)
	}
	for _, want := range []string{"Operating Mode", "Version Management", "Node.js", "Enforce permission model", "shim", "22.0.0"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("Execute(env) stdout = %q, want substring %q", stdout, want)
		}
	}
}

func TestCLIEnvJSON(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("env", "--json")
	if err != nil {
		t.Fatalf("Execute(env --json) error = %v stderr = %q", err, stderr)
	}

	var out struct {
		Operations struct {
			Mode          string `json:"mode"`
			Root          string `json:"version_root"`
			ActiveVersion string `json:"version_active"`
			Status        string `json:"status"`
		} `json:"operations"`
		Node struct {
			EnforcePermissionModel        bool   `json:"enforce_permission_model"`
			FreezeV8GlobalObjects         bool   `json:"freeze_v8_global_objects"`
			DisableEvalAndStringExecution bool   `json:"disable_eval_and_string_execution"`
			Enforced                      bool   `json:"enforced"`
			EnforcementNote               string `json:"enforcement_note"`
		} `json:"node"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v stdout = %q", err, stdout)
	}
	if out.Operations.Mode != "shim" {
		t.Fatalf("operations.mode = %q, want shim", out.Operations.Mode)
	}
	if out.Operations.ActiveVersion != "22.0.0" {
		t.Fatalf("operations.version_active = %q, want 22.0.0", out.Operations.ActiveVersion)
	}
	if out.Operations.Status != "on" {
		t.Fatalf("operations.status = %q, want on", out.Operations.Status)
	}
	if !out.Node.Enforced {
		t.Fatalf("node.enforced = false, want true in shim mode")
	}
	if out.Node.EnforcementNote != "" {
		t.Fatalf("node.enforcement_note = %q, want empty in shim mode", out.Node.EnforcementNote)
	}

	wantRoot := strings.ToLower(filepath.ToSlash(sb.InstallRoot))
	gotRoot := strings.ToLower(filepath.ToSlash(out.Operations.Root))
	if gotRoot != wantRoot {
		t.Fatalf("operations.version_root = %q, want %q", out.Operations.Root, sb.InstallRoot)
	}
}

func TestCLIListEmpty(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("list")
	if err != nil {
		t.Fatalf("Execute(list) error = %v stderr = %q", err, stderr)
	}
	if strings.TrimSpace(stdout) != "No versions installed." {
		t.Fatalf("Execute(list) stdout = %q, want empty message", stdout)
	}
}

func TestCLIListSeeded(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	if err := sb.WriteSetting("active_version", "22.0.0"); err != nil {
		t.Fatalf("WriteSetting(active_version) error = %v", err)
	}

	stdout, stderr, err := sb.Execute("list")
	if err != nil {
		t.Fatalf("Execute(list) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(list) stdout = %q, want seeded version", stdout)
	}
	if !strings.Contains(stdout, "(default)") {
		t.Fatalf("Execute(list) stdout = %q, want default marker", stdout)
	}
}

func TestCLIListJSON(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)

	stdout, stderr, err := sb.Execute("list", "--json")
	if err != nil {
		t.Fatalf("Execute(list --json) error = %v stderr = %q", err, stderr)
	}

	var out []struct {
		Version   string `json:"version"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v stdout = %q", err, stdout)
	}
	if len(out) != 1 {
		t.Fatalf("list JSON len = %d, want 1", len(out))
	}
	if out[0].Version != "22.0.0" || !out[0].Installed {
		t.Fatalf("list JSON[0] = %+v, want version 22.0.0 installed=true", out[0])
	}
}

func TestCLIListMajorFilter(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)
	sb.SeedVersion("20.0.0", nil)

	stdout, stderr, err := sb.Execute("list", "22")
	if err != nil {
		t.Fatalf("Execute(list 22) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(list 22) stdout = %q, want v22", stdout)
	}
	if strings.Contains(stdout, "20.0.0") {
		t.Fatalf("Execute(list 22) stdout = %q, should not include v20", stdout)
	}
}

func TestCLIListAliasLS(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedVersion("22.0.0", nil)

	stdout, stderr, err := sb.Execute("ls")
	if err != nil {
		t.Fatalf("Execute(ls) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(ls) stdout = %q, want seeded version", stdout)
	}
}

func TestCLIUnknownCommand(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.Execute("not-a-real-command")
	if err == nil {
		t.Fatal("Execute(not-a-real-command) expected error, got nil")
	}
	combined := strings.ToLower(err.Error() + stderr)
	if !strings.Contains(combined, "usage") &&
		!strings.Contains(combined, "unknown") &&
		!strings.Contains(combined, "unexpected argument") {
		t.Fatalf("Execute(not-a-real-command) error = %v stderr = %q, want usage hint", err, stderr)
	}
}

func TestCLIInstallTypoAliasParses(t *testing.T) {
	sb := clitest.NewSandbox(t)

	_, stderr, err := sb.Execute("instal")
	if err == nil {
		t.Fatal("Execute(instal) expected run error without version, got nil")
	}
	if strings.Contains(strings.ToLower(err.Error()), "unknown command") {
		t.Fatalf("Execute(instal) error = %v, typo alias should resolve to install", err)
	}
	_ = stderr
}

func TestCLIConfigListRouting(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("config", "list")
	if err != nil {
		t.Fatalf("Execute(config list) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout+stderr, "root") && !strings.Contains(stdout+stderr, "mode") {
		t.Fatalf("Execute(config list) output = stdout:%q stderr:%q, want config keys", stdout, stderr)
	}
}
