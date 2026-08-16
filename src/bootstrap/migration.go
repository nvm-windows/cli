package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	winreg "golang.org/x/sys/windows/registry"
)

var (
	legacyUserUninstallKeyPaths = []string{
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\40078385-F676-4C61-9A9C-F9028599D6D3_is1`,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\nvm_is1`,
	}
	legacyNvmAppPathKey         = `Software\Microsoft\Windows\CurrentVersion\App Paths\nvm.exe`
	legacySyncAppPathKey        = `Software\Microsoft\Windows\CurrentVersion\App Paths\sync.exe`
	legacyShellRegistrationBase = `Software\Classes\nvm`
	legacySyncTaskNames         = []string{"NVM for Windows Sync", "NVM Sync"}
	queryScheduledTaskXML       = defaultScheduledTaskQuery
	deleteScheduledTask         = defaultScheduledTaskDelete
)

func cleanupLegacyUserPayload(dataRoot string) error {
	programRoot, err := ProgramRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve program root: %w", err)
	}

	legacyNvmExe := filepath.Join(dataRoot, "nvm.exe")
	legacySyncExe := filepath.Join(dataRoot, "utils", "sync.exe")

	// Per-user/community installs live under the data root. Never treat that live
	// payload as leftover "legacy" files — deleting nvm.exe would Access-Deny the
	// running binary and break bootstrap.
	if sameLegacyPath(programRoot, dataRoot) {
		if err := removeLegacyCurrentUserEnv(dataRoot); err != nil {
			return err
		}
		return nil
	}

	if err := removeLegacyCurrentUserEnv(dataRoot); err != nil {
		return err
	}

	if err := removeLegacySyncTasks(legacySyncExe); err != nil {
		return err
	}

	if err := removeLegacyAppPathKey(legacyNvmAppPathKey, legacyNvmExe); err != nil {
		return err
	}
	if err := removeLegacyAppPathKey(legacySyncAppPathKey, legacySyncExe); err != nil {
		return err
	}
	if err := removeLegacyShellRegistration(legacyShellRegistrationBase, legacyNvmExe); err != nil {
		return err
	}
	if err := removeLegacyUninstallEntries(dataRoot); err != nil {
		return err
	}

	for _, path := range []string{
		legacyNvmExe,
		filepath.Join(dataRoot, "utils"),
		filepath.Join(dataRoot, ".icons"),
	} {
		if err := removeLegacyPath(path); err != nil {
			return err
		}
	}

	return nil
}

func removeLegacyCurrentUserEnv(dataRoot string) error {
	key, err := winreg.OpenKey(winreg.CURRENT_USER, `Environment`, winreg.QUERY_VALUE|winreg.SET_VALUE)
	if err != nil {
		if err == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to open current-user environment key: %w", err)
	}
	defer key.Close()

	nvmHome, _, valueErr := key.GetStringValue("NVM_HOME")
	if valueErr != nil {
		if valueErr == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to read current-user NVM_HOME: %w", valueErr)
	}
	if !valueReferencesPath(nvmHome, dataRoot) {
		return nil
	}

	if err := key.DeleteValue("NVM_HOME"); err != nil && err != winreg.ErrNotExist {
		return fmt.Errorf("failed to delete current-user NVM_HOME: %w", err)
	}

	return nil
}

func removeLegacyPath(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect legacy path %s: %w", path, err)
	}

	if err := os.RemoveAll(path); err != nil {
		if isBusyLegacyPathError(err) {
			fmt.Fprintf(os.Stderr, "nvm: warning: skipped locked legacy path %s: %v\n", path, err)
			return nil
		}
		return fmt.Errorf("failed to remove legacy path %s: %w", path, err)
	}

	return nil
}

func isBusyLegacyPathError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		return true
	}
	var errno syscall.Errno
	if errors.As(err, &errno) && errno == syscall.ERROR_ACCESS_DENIED {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "access is denied")
}

func sameLegacyPath(a, b string) bool {
	left := normalizePathMatch(a)
	right := normalizePathMatch(b)
	return left != "" && left == right
}

func removeLegacyUninstallEntries(dataRoot string) error {
	for _, keyPath := range legacyUserUninstallKeyPaths {
		matches, err := uninstallEntryMatchesPath(keyPath, dataRoot)
		if err != nil {
			return err
		}
		if !matches {
			continue
		}

		if err := deleteCurrentUserKey(keyPath); err != nil {
			return fmt.Errorf("failed to remove legacy uninstall key %s: %w", keyPath, err)
		}
	}

	return nil
}

func uninstallEntryMatchesPath(keyPath, expectedPath string) (bool, error) {
	key, err := winreg.OpenKey(winreg.CURRENT_USER, keyPath, winreg.QUERY_VALUE)
	if err != nil {
		if err == winreg.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("failed to open uninstall key %s: %w", keyPath, err)
	}
	defer key.Close()

	for _, valueName := range []string{"InstallLocation", "UninstallString", "DisplayIcon"} {
		value, _, valueErr := key.GetStringValue(valueName)
		if valueErr != nil {
			if valueErr == winreg.ErrNotExist {
				continue
			}
			return false, fmt.Errorf("failed to read uninstall key value %s from %s: %w", valueName, keyPath, valueErr)
		}

		if valueReferencesPath(value, expectedPath) {
			return true, nil
		}
	}

	return false, nil
}

func removeLegacyAppPathKey(keyPath, expectedPath string) error {
	key, err := winreg.OpenKey(winreg.CURRENT_USER, keyPath, winreg.QUERY_VALUE)
	if err != nil {
		if err == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to open app path key %s: %w", keyPath, err)
	}

	value, _, valueErr := key.GetStringValue("")
	key.Close()
	if valueErr != nil {
		if valueErr == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to read app path key %s: %w", keyPath, valueErr)
	}
	if !valueReferencesPath(value, expectedPath) {
		return nil
	}

	if err := deleteCurrentUserKey(keyPath); err != nil {
		return fmt.Errorf("failed to remove app path key %s: %w", keyPath, err)
	}

	return nil
}

func removeLegacyShellRegistration(baseKey, expectedPath string) error {
	commandKey := baseKey + `\shell\open\command`
	key, err := winreg.OpenKey(winreg.CURRENT_USER, commandKey, winreg.QUERY_VALUE)
	if err != nil {
		if err == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to open shell registration key %s: %w", commandKey, err)
	}

	value, _, valueErr := key.GetStringValue("")
	key.Close()
	if valueErr != nil {
		if valueErr == winreg.ErrNotExist {
			return nil
		}
		return fmt.Errorf("failed to read shell registration key %s: %w", commandKey, valueErr)
	}
	if !valueReferencesPath(value, expectedPath) {
		return nil
	}

	for _, keyPath := range []string{
		commandKey,
		baseKey + `\shell\open`,
		baseKey + `\shell`,
		baseKey,
	} {
		if err := deleteCurrentUserKey(keyPath); err != nil {
			return fmt.Errorf("failed to remove shell registration key %s: %w", keyPath, err)
		}
	}

	return nil
}

func removeLegacySyncTasks(expectedPath string) error {
	for _, taskName := range legacySyncTaskNames {
		xml, err := queryScheduledTaskXML(taskName)
		if err != nil {
			if isScheduledTaskMissing(err, xml) {
				continue
			}
			return fmt.Errorf("failed to inspect scheduled task %q: %w", taskName, err)
		}

		if !valueReferencesPath(xml, expectedPath) {
			continue
		}

		if err := deleteScheduledTask(taskName); err != nil {
			if isScheduledTaskMissing(err, "") {
				continue
			}
			return fmt.Errorf("failed to remove scheduled task %q: %w", taskName, err)
		}
	}

	return nil
}

func defaultScheduledTaskQuery(taskName string) (string, error) {
	output, err := exec.Command("schtasks", "/Query", "/TN", taskName, "/XML").CombinedOutput()
	return string(output), err
}

func defaultScheduledTaskDelete(taskName string) error {
	output, err := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, trimmed)
	}

	return nil
}

func deleteCurrentUserKey(keyPath string) error {
	err := winreg.DeleteKey(winreg.CURRENT_USER, keyPath)
	if err != nil && err != winreg.ErrNotExist {
		return err
	}

	return nil
}

func isScheduledTaskMissing(err error, output string) bool {
	text := strings.ToLower(strings.TrimSpace(err.Error() + " " + output))
	return strings.Contains(text, "cannot find the file specified") ||
		strings.Contains(text, "cannot find the task") ||
		strings.Contains(text, "the system cannot find the file specified")
}

func valueReferencesPath(value, expectedPath string) bool {
	normalizedValue := normalizePathMatch(value)
	normalizedExpected := normalizePathMatch(expectedPath)
	if normalizedValue == "" || normalizedExpected == "" {
		return false
	}

	return strings.Contains(normalizedValue, normalizedExpected)
}

func normalizePathMatch(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	normalized = strings.ReplaceAll(normalized, `\\`, `\`)
	normalized = strings.Trim(normalized, `"`)
	return strings.TrimRight(normalized, `\`)
}
