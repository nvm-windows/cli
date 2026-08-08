package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestExpand(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_EXPAND_VAR", "expanded_value")
	defer os.Unsetenv("TEST_EXPAND_VAR")

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Expand single variable",
			input:    "%TEST_EXPAND_VAR%",
			contains: "expanded_value",
		},
		{
			name:     "Expand variable in path",
			input:    "%TEST_EXPAND_VAR%\\subfolder",
			contains: "expanded_value\\subfolder",
		},
		{
			name:     "Unknown variable remains unchanged",
			input:    "%UNKNOWN_VAR%\\path",
			contains: "%UNKNOWN_VAR%\\path",
		},
		{
			name:     "No variables",
			input:    "C:\\Users\\test",
			contains: "C:\\Users\\test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expand(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expand() = %q, want it to contain %q", result, tt.contains)
			}
		})
	}
}

func TestExpandRealVars(t *testing.T) {
	// Test with actual system variables
	tests := []struct {
		name  string
		input string
	}{
		{name: "APPDATA variable", input: "%APPDATA%"},
		{name: "LOCALAPPDATA variable", input: "%LOCALAPPDATA%"},
		{name: "USERPROFILE variable", input: "%USERPROFILE%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expand(tt.input)
			// Result should either be expanded (not contain %) or unchanged if var doesn't exist
			if strings.Contains(result, "%") {
				// It's OK if the variable doesn't exist, it should remain as-is
				if result != tt.input {
					t.Errorf("expand() = %q, expected either %q or expanded value", result, tt.input)
				}
			}
		})
	}
}

func TestCurrentUserDataRootUsesLocalAppData(t *testing.T) {
	got, ok := currentUserDataRoot(func(name string) (string, bool) {
		if name == localAppDataEnvName {
			return `C:\Users\test\AppData\Local`, true
		}

		return "", false
	})
	if !ok {
		t.Fatal("currentUserDataRoot() should resolve a normal LOCALAPPDATA path")
	}

	want := filepath.Join(`C:\Users\test\AppData\Local`, "Author Software", "nvm")
	if got != want {
		t.Fatalf("currentUserDataRoot() = %q, want %q", got, want)
	}
}

func TestCurrentUserDataRootSkipsServiceProfiles(t *testing.T) {
	_, ok := currentUserDataRoot(func(name string) (string, bool) {
		if name == localAppDataEnvName {
			return `C:\Windows\System32\config\systemprofile\AppData\Local`, true
		}

		return "", false
	})
	if ok {
		t.Fatal("currentUserDataRoot() should skip service profile LOCALAPPDATA paths")
	}
}

func TestCleanupCurrentUserAppDataRemovesRoot(t *testing.T) {
	localAppData := t.TempDir()
	dataRoot := filepath.Join(localAppData, "Author Software", "nvm")
	if err := os.MkdirAll(filepath.Join(dataRoot, ".sync"), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dataRoot, err)
	}
	if err := os.WriteFile(filepath.Join(dataRoot, ".sync", "seed.dll"), []byte("seed"), 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filepath.Join(dataRoot, ".sync", "seed.dll"), err)
	}

	originalValue, hadOriginal := os.LookupEnv(localAppDataEnvName)
	if err := os.Setenv(localAppDataEnvName, localAppData); err != nil {
		t.Fatalf("Setenv(%q) error = %v", localAppDataEnvName, err)
	}
	oldDelete := deleteScheduledTask
	deleteScheduledTask = func(string) error { return nil }
	t.Cleanup(func() {
		deleteScheduledTask = oldDelete
		if hadOriginal {
			_ = os.Setenv(localAppDataEnvName, originalValue)
			return
		}

		_ = os.Unsetenv(localAppDataEnvName)
	})

	if err := CleanupCurrentUserAppData(); err != nil {
		t.Fatalf("CleanupCurrentUserAppData() error = %v", err)
	}

	if _, err := os.Stat(dataRoot); !os.IsNotExist(err) {
		t.Fatalf("data root %q should be removed, stat err = %v", dataRoot, err)
	}
}

func TestCleanupCurrentUserAppDataRemovesSyncScheduledTasks(t *testing.T) {
	localAppData := t.TempDir()
	originalValue, hadOriginal := os.LookupEnv(localAppDataEnvName)
	if err := os.Setenv(localAppDataEnvName, localAppData); err != nil {
		t.Fatalf("Setenv(%q) error = %v", localAppDataEnvName, err)
	}

	oldDelete := deleteScheduledTask
	deleted := make([]string, 0, 2)
	deleteScheduledTask = func(taskName string) error {
		deleted = append(deleted, taskName)
		return nil
	}
	t.Cleanup(func() {
		deleteScheduledTask = oldDelete
		if hadOriginal {
			_ = os.Setenv(localAppDataEnvName, originalValue)
			return
		}
		_ = os.Unsetenv(localAppDataEnvName)
	})

	if err := CleanupCurrentUserAppData(); err != nil {
		t.Fatalf("CleanupCurrentUserAppData() error = %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("deleted tasks = %#v, want both sync task names", deleted)
	}
	if deleted[0] != "NVM for Windows Sync" || deleted[1] != "NVM Sync" {
		t.Fatalf("deleted tasks = %#v", deleted)
	}
}

func TestRemoveSyncScheduledTasksDeletesKnownTaskNames(t *testing.T) {
	oldDelete := deleteScheduledTask
	deleted := make([]string, 0, 2)
	deleteScheduledTask = func(taskName string) error {
		deleted = append(deleted, taskName)
		return nil
	}
	t.Cleanup(func() { deleteScheduledTask = oldDelete })

	if err := RemoveSyncScheduledTasks(); err != nil {
		t.Fatalf("RemoveSyncScheduledTasks() error = %v", err)
	}

	if got, want := len(deleted), len(syncScheduledTaskNames); got != want {
		t.Fatalf("deleted count = %d, want %d (%v)", got, want, deleted)
	}
	for i, name := range syncScheduledTaskNames {
		if deleted[i] != name {
			t.Fatalf("deleted[%d] = %q, want %q", i, deleted[i], name)
		}
	}
}

func TestCleanupCurrentUserAppDataIgnoresMissingSyncScheduledTasks(t *testing.T) {
	localAppData := t.TempDir()
	originalValue, hadOriginal := os.LookupEnv(localAppDataEnvName)
	if err := os.Setenv(localAppDataEnvName, localAppData); err != nil {
		t.Fatalf("Setenv(%q) error = %v", localAppDataEnvName, err)
	}

	oldDelete := deleteScheduledTask
	deleteScheduledTask = func(string) error {
		return fmt.Errorf("exit status 1: ERROR: The system cannot find the file specified.")
	}
	t.Cleanup(func() {
		deleteScheduledTask = oldDelete
		if hadOriginal {
			_ = os.Setenv(localAppDataEnvName, originalValue)
			return
		}
		_ = os.Unsetenv(localAppDataEnvName)
	})

	if err := CleanupCurrentUserAppData(); err != nil {
		t.Fatalf("CleanupCurrentUserAppData() error = %v", err)
	}
}

func TestCleanupCurrentUserAppDataRemovesNodeWindowsAppsEntries(t *testing.T) {
	localAppData := t.TempDir()

	originalValue, hadOriginal := os.LookupEnv(localAppDataEnvName)
	if err := os.Setenv(localAppDataEnvName, localAppData); err != nil {
		t.Fatalf("Setenv(%q) error = %v", localAppDataEnvName, err)
	}
	oldDelete := deleteScheduledTask
	deleteScheduledTask = func(string) error { return nil }
	t.Cleanup(func() {
		deleteScheduledTask = oldDelete
		if hadOriginal {
			_ = os.Setenv(localAppDataEnvName, originalValue)
			return
		}

		_ = os.Unsetenv(localAppDataEnvName)
	})

	matchingKeyPath := windowsAppsUninstallRoot + `\nvm4w-node-v99.99.99`
	unrelatedKeyPath := windowsAppsUninstallRoot + `\some-other-app`

	matchingKey, _, err := registry.CreateKey(registry.CURRENT_USER, matchingKeyPath, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("CreateKey(%q) error = %v", matchingKeyPath, err)
	}
	if err := matchingKey.SetStringValue("DisplayName", "Node.js v99.99.99 via nvm-windows"); err != nil {
		matchingKey.Close()
		t.Fatalf("SetStringValue(DisplayName) error = %v", err)
	}
	matchingKey.Close()

	unrelatedKey, _, err := registry.CreateKey(registry.CURRENT_USER, unrelatedKeyPath, registry.SET_VALUE)
	if err != nil {
		_ = deleteRegistryTree(registry.CURRENT_USER, matchingKeyPath)
		t.Fatalf("CreateKey(%q) error = %v", unrelatedKeyPath, err)
	}
	if err := unrelatedKey.SetStringValue("DisplayName", "Some Other App"); err != nil {
		unrelatedKey.Close()
		_ = deleteRegistryTree(registry.CURRENT_USER, matchingKeyPath)
		t.Fatalf("SetStringValue(DisplayName) error = %v", err)
	}
	unrelatedKey.Close()

	t.Cleanup(func() {
		_ = deleteRegistryTree(registry.CURRENT_USER, matchingKeyPath)
		_ = deleteRegistryTree(registry.CURRENT_USER, unrelatedKeyPath)
	})

	if err := CleanupCurrentUserAppData(); err != nil {
		t.Fatalf("CleanupCurrentUserAppData() error = %v", err)
	}

	if _, err := registry.OpenKey(registry.CURRENT_USER, matchingKeyPath, registry.QUERY_VALUE); err == nil {
		t.Fatalf("expected %q to be removed", matchingKeyPath)
	}

	if key, err := registry.OpenKey(registry.CURRENT_USER, unrelatedKeyPath, registry.QUERY_VALUE); err != nil {
		t.Fatalf("expected unrelated key %q to remain: %v", unrelatedKeyPath, err)
	} else {
		key.Close()
	}
}
