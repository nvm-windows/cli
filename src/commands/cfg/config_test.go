package cfg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"nvm/constant"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	prefs "common/preferences"
	"common/settings"

	"github.com/alecthomas/kong"
)

const commandTestRegistryRoot = "HKCU/Software/NVMTest/config_set_test"

func TestMain(m *testing.M) {
	prefs.ROOT = commandTestRegistryRoot
	prefs.ROOTS = []string{prefs.ROOT}
	code := m.Run()
	exec.Command("reg", "delete", `HKCU\Software\NVMTest`, "/f").Run() //nolint:errcheck
	os.Exit(code)
}

func runSetCfg(t *testing.T, pairs ...string) error {
	t.Helper()
	return (&Set{Pairs: pairs}).Run()
}

func getSetting(t *testing.T, name string) interface{} {
	t.Helper()
	value, err := settings.Get(name)
	if err != nil {
		t.Fatalf("Get(%q) unexpected error: %v", name, err)
	}
	return value
}

func expectRunErrorContains(t *testing.T, pairs []string, want string) {
	t.Helper()
	err := runSetCfg(t, pairs...)
	if err == nil {
		t.Fatalf("Run(%v) expected error containing %q, got nil", pairs, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Run(%v) expected error containing %q, got %q", pairs, want, err.Error())
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = writer

	errCh := make(chan error, 1)
	var buffer bytes.Buffer
	go func() {
		_, copyErr := io.Copy(&buffer, reader)
		errCh <- copyErr
	}()

	runErr := fn()
	writer.Close()
	os.Stdout = oldStdout
	copyErr := <-errCh
	reader.Close()
	if copyErr != nil {
		t.Fatalf("failed capturing stdout: %v", copyErr)
	}

	return buffer.String(), runErr
}

func TestSet_RunRejectsMalformedAssignments(t *testing.T) {
	for _, pair := range []string{"mode", "=shim", "root", "proxy"} {
		t.Run(fmt.Sprintf("%q", pair), func(t *testing.T) {
			expectRunErrorContains(t, []string{pair}, "expected key=value")
		})
	}
}

func TestSet_RunRejectsUnknownKeys(t *testing.T) {
	expectRunErrorContains(t, []string{"bogus=value"}, `invalid configuration key "bogus"`)
}

func TestSet_RunPersistsValidSettings(t *testing.T) {
	root := filepath.Join(os.TempDir(), "nvm_config_set_valid_root")
	defer os.RemoveAll(root)

	err := runSetCfg(t,
		"mode=junction",
		"root="+root,
		"proxy=https://proxy.example.com:8080",
		"node_mirror=https://nodejs.org/dist,https://mirror.example.com/node",
		"npm_mirror=https://github.com/npm/cli/archive/",
		"active_version=22.0.0",
		"auto_use=1",
		"auto_install=0",
	)
	if err != nil {
		t.Fatalf("Run(valid pairs) unexpected error: %v", err)
	}

	if got := getSetting(t, "mode"); got != "junction" {
		t.Fatalf("mode: expected %q, got %v", "junction", got)
	}
	if got := getSetting(t, "root"); got != root {
		t.Fatalf("root: expected %q, got %v", root, got)
	}
	if got := getSetting(t, "proxy"); got != "https://proxy.example.com:8080" {
		t.Fatalf("proxy: unexpected value %v", got)
	}
	if got := getSetting(t, "active_version"); got != "22.0.0" {
		t.Fatalf("active_version: expected %q, got %v", "22.0.0", got)
	}
	if got := getSetting(t, "auto_use"); got != true {
		t.Fatalf("auto_use: expected true, got %v", got)
	}
	if got := getSetting(t, "auto_install"); got != false {
		t.Fatalf("auto_install: expected false, got %v", got)
	}

	if _, err := os.Stat(root); err != nil {
		t.Fatalf("root directory was not created: %v", err)
	}
}

func TestSet_RunRejectsInvalidMode(t *testing.T) {
	expectRunErrorContains(t, []string{"mode=invalid"}, "must be one of: shim, symlink, junction")
}

func TestSet_RunRejectsInvalidRoot(t *testing.T) {
	expectRunErrorContains(t, []string{"root=   "}, "root must be a non-empty path")
}

func TestSet_RunRejectsInvalidProxy(t *testing.T) {
	expectRunErrorContains(t, []string{"proxy=not-a-url"}, "is not a valid URL")
}

func TestSet_RunRejectsInvalidNodeMirror(t *testing.T) {
	expectRunErrorContains(t, []string{"node_mirror=https://nodejs.org/dist,not-a-url"}, "is not a valid URL")
}

func TestSet_RunRejectsInvalidNpmMirror(t *testing.T) {
	expectRunErrorContains(t, []string{"npm_mirror=bad-value"}, "is not a valid URL")
}

func TestSet_RunRejectsInvalidActiveVersion(t *testing.T) {
	expectRunErrorContains(t, []string{"active_version=22"}, "is not a valid semantic version")
}

func TestSet_RunRejectsInvalidAutoUse(t *testing.T) {
	expectRunErrorContains(t, []string{"auto_use=true"}, "auto_use must be 0 or 1")
}

func TestSet_RunRejectsInvalidAutoInstall(t *testing.T) {
	expectRunErrorContains(t, []string{"auto_install=2"}, "auto_install must be 0 or 1")
}

func TestSet_RunAggregatesMultipleValidationErrors(t *testing.T) {
	err := runSetCfg(t, "proxy=bad", "active_version=22")
	if err == nil {
		t.Fatal("expected aggregated validation error, got nil")
	}

	message := err.Error()
	if !strings.Contains(message, "errors updating configuration") {
		t.Fatalf("expected aggregate error prefix, got %q", message)
	}
	if !strings.Contains(message, "proxy \"bad\" is not a valid URL") {
		t.Fatalf("expected proxy validation error in %q", message)
	}
	if !strings.Contains(message, "active_version \"22\" is not a valid semantic version") {
		t.Fatalf("expected active_version validation error in %q", message)
	}
}

func TestSet_RunLastValueWinsForDuplicateKeys(t *testing.T) {
	if err := runSetCfg(t, "mode=shim", "mode=symlink"); err != nil {
		t.Fatalf("Run(duplicate keys) unexpected error: %v", err)
	}

	if got := getSetting(t, "mode"); got != "symlink" {
		t.Fatalf("expected duplicate key handling to keep last value, got %v", got)
	}
}

func TestSet_RunPersistsAccessTokenAndRedactsOutput(t *testing.T) {
	const accessToken = "header.payload.signature"

	output, err := captureStdout(t, func() error {
		return runSetCfg(t, "access_token="+accessToken)
	})
	if err != nil {
		t.Fatalf("Set.Run(access_token) unexpected error: %v", err)
	}

	if got := getSetting(t, "access_token"); got != accessToken {
		t.Fatalf("expected access_token to persist raw value, got %v", got)
	}

	if strings.Contains(output, accessToken) {
		t.Fatalf("expected access_token output to redact the token, got %q", output)
	}
	if !strings.Contains(output, "access_token set to:") {
		t.Fatalf("expected access_token confirmation output, got %q", output)
	}
	if !strings.Contains(output, "(redacted)") {
		t.Fatalf("expected access_token output to show redaction marker, got %q", output)
	}
}

func TestGet_RunSingleValue(t *testing.T) {
	root := filepath.Join(os.TempDir(), "nvm_config_get_root")
	defer os.RemoveAll(root)

	if err := runSetCfg(t, "root="+root); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&Get{Name: []string{"root"}}).Run()
	})
	if err != nil {
		t.Fatalf("Get.Run() unexpected error: %v", err)
	}

	if strings.TrimSpace(output) != root {
		t.Fatalf("expected %q, got %q", root, strings.TrimSpace(output))
	}
}

func TestGet_RunRedactsAccessToken(t *testing.T) {
	const accessToken = "header.payload.signature"
	if err := runSetCfg(t, "access_token="+accessToken); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&Get{Name: []string{"access_token"}}).Run()
	})
	if err != nil {
		t.Fatalf("Get.Run(access_token) unexpected error: %v", err)
	}

	trimmed := strings.TrimSpace(output)
	if trimmed != "(redacted)" {
		t.Fatalf("expected redacted access token output, got %q", trimmed)
	}
	if strings.Contains(trimmed, accessToken) {
		t.Fatalf("expected access token text output to be redacted, got %q", trimmed)
	}

	jsonOutput, err := captureStdout(t, func() error {
		return (&Get{Name: []string{"access_token"}, FlagJSON: constant.FlagJSON{JSON: true}}).Run()
	})
	if err != nil {
		t.Fatalf("Get.Run(access_token JSON) unexpected error: %v", err)
	}

	data := map[string]interface{}{}
	if err := json.Unmarshal([]byte(jsonOutput), &data); err != nil {
		t.Fatalf("failed to decode JSON output %q: %v", jsonOutput, err)
	}

	if data["access_token"] != "(redacted)" {
		t.Fatalf("expected JSON access_token to be redacted, got %#v", data["access_token"])
	}
}

func TestGet_RunMultipleValues(t *testing.T) {
	if err := runSetCfg(t, "mode=symlink", "auto_use=1"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&Get{Name: []string{"mode", "auto_use"}}).Run()
	})
	if err != nil {
		t.Fatalf("Get.Run() unexpected error: %v", err)
	}

	if !strings.Contains(output, "mode: symlink") {
		t.Fatalf("expected mode output, got %q", output)
	}
	if !strings.Contains(output, "auto_use: true") {
		t.Fatalf("expected auto_use output, got %q", output)
	}
}

func TestGet_RunJSON(t *testing.T) {
	if err := runSetCfg(t, "mode=junction", "auto_install=0"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&Get{Name: []string{"mode", "auto_install"}, FlagJSON: constant.FlagJSON{JSON: true}}).Run()
	})
	if err != nil {
		t.Fatalf("Get.Run() unexpected error: %v", err)
	}

	data := map[string]interface{}{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Fatalf("failed to decode JSON output %q: %v", output, err)
	}

	if data["mode"] != "junction" {
		t.Fatalf("expected mode=junction, got %#v", data["mode"])
	}
	if data["auto_install"] != false {
		t.Fatalf("expected auto_install=false, got %#v", data["auto_install"])
	}
}

func TestDel_RunDeletesSetting(t *testing.T) {
	if err := runSetCfg(t, "active_version=22.0.0"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if got := getSetting(t, "active_version"); got != "22.0.0" {
		t.Fatalf("expected active_version to be set before delete, got %v", got)
	}

	if err := (&Del{Name: "active_version", Quiet: true}).Run(); err != nil {
		t.Fatalf("Del.Run() unexpected error: %v", err)
	}

	if got := getSetting(t, "active_version"); got != nil {
		t.Fatalf("expected deleted value to fall back to nil default, got %v", got)
	}
}

func TestDel_RunUnknownSettingFails(t *testing.T) {
	if err := (&Del{Name: "missing"}).Run(); err == nil {
		t.Fatal("expected delete of unknown setting to fail")
	}
}

func TestList_RunPrintsAllSettings(t *testing.T) {
	root := filepath.Join(os.TempDir(), "nvm_config_list_root")
	defer os.RemoveAll(root)

	if err := runSetCfg(t,
		"root="+root,
		"active_version=20.0.0",
		"cache_downloads=1",
		"auto_use=1",
		"auto_install=0",
		"access_token=header.payload.signature",
	); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&List{}).Run(kong.Vars{"app": "nvm"})
	})
	if err != nil {
		t.Fatalf("List.Run() unexpected error: %v", err)
	}

	checks := []string{
		"cache_downloads",
		"true",
		"auto_use",
		"true",
		"auto_install",
		"false",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected list output to contain %q, got %q", check, output)
		}
	}

	hiddenChecks := []string{"root", root, "active_version", "20.0.0", "access_token", "header.payload.signature"}
	for _, check := range hiddenChecks {
		if strings.Contains(output, check) {
			t.Fatalf("expected list output to hide %q, got %q", check, output)
		}
	}
}

func TestList_RunPrintsJSONWhenEnabled(t *testing.T) {
	if err := runSetCfg(t,
		"cache_downloads=1",
		"auto_use=1",
		"auto_install=0",
	); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	output, err := captureStdout(t, func() error {
		return (&List{FlagJSON: constant.FlagJSON{JSON: true}}).Run(kong.Vars{"app": "nvm"})
	})
	if err != nil {
		t.Fatalf("List.Run(JSON) unexpected error: %v", err)
	}

	data := map[string]interface{}{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Fatalf("failed to decode JSON output %q: %v", output, err)
	}

	if data["cache_downloads"] != true {
		t.Fatalf("expected cache_downloads=true, got %#v", data["cache_downloads"])
	}
	if data["auto_install"] != false {
		t.Fatalf("expected auto_install=false, got %#v", data["auto_install"])
	}
	if _, ok := data["root"]; ok {
		t.Fatalf("expected root to be hidden in JSON output, got %#v", data["root"])
	}
	if _, ok := data["access_token"]; ok {
		t.Fatalf("expected access_token to be hidden in JSON output, got %#v", data["access_token"])
	}
}
