package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureSyncScheduledTaskCreatesWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	utilsDir := filepath.Join(tmp, "utils")
	if err := os.MkdirAll(utilsDir, 0o755); err != nil {
		t.Fatalf("mkdir utils: %v", err)
	}
	syncExe := filepath.Join(utilsDir, "sync.exe")
	if err := os.WriteFile(syncExe, []byte("sync"), 0o644); err != nil {
		t.Fatalf("write sync.exe: %v", err)
	}

	oldProgramRoot := programRootOverride
	oldQuery := queryScheduledTaskXML
	oldCreate := createScheduledTask
	oldChange := changeScheduledTask
	t.Cleanup(func() {
		programRootOverride = oldProgramRoot
		queryScheduledTaskXML = oldQuery
		createScheduledTask = oldCreate
		changeScheduledTask = oldChange
	})

	programRootOverride = tmp
	created := false
	enabled := false
	queryScheduledTaskXML = func(taskName string) (string, error) {
		if taskName != syncScheduledTaskName {
			t.Fatalf("query task = %q", taskName)
		}
		return "", errors.New("ERROR: The system cannot find the file specified.")
	}
	createScheduledTask = func(taskName, exe string) error {
		created = true
		if taskName != syncScheduledTaskName {
			t.Fatalf("create task = %q", taskName)
		}
		if !strings.EqualFold(exe, syncExe) {
			t.Fatalf("create exe = %q, want %q", exe, syncExe)
		}
		return nil
	}
	changeScheduledTask = func(taskName string, on bool) error {
		if taskName != syncScheduledTaskName {
			t.Fatalf("change task = %q", taskName)
		}
		if !on {
			t.Fatal("expected enable")
		}
		enabled = true
		return nil
	}

	ensureSyncScheduledTask()
	if !created {
		t.Fatal("expected scheduled task creation")
	}
	if !enabled {
		t.Fatal("expected scheduled task enable")
	}
}

func TestEnsureSyncScheduledTaskSkipsCreateWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	utilsDir := filepath.Join(tmp, "utils")
	if err := os.MkdirAll(utilsDir, 0o755); err != nil {
		t.Fatalf("mkdir utils: %v", err)
	}
	syncExe := filepath.Join(utilsDir, "sync.exe")
	if err := os.WriteFile(syncExe, []byte("sync"), 0o644); err != nil {
		t.Fatalf("write sync.exe: %v", err)
	}

	oldProgramRoot := programRootOverride
	oldQuery := queryScheduledTaskXML
	oldCreate := createScheduledTask
	oldChange := changeScheduledTask
	t.Cleanup(func() {
		programRootOverride = oldProgramRoot
		queryScheduledTaskXML = oldQuery
		createScheduledTask = oldCreate
		changeScheduledTask = oldChange
	})

	programRootOverride = tmp
	enabled := false
	queryScheduledTaskXML = func(string) (string, error) {
		return `<Task><Actions><Exec><Command>` + syncExe + `</Command></Exec></Actions></Task>`, nil
	}
	createScheduledTask = func(string, string) error {
		t.Fatal("createScheduledTask should not run when task already points at sync.exe")
		return nil
	}
	changeScheduledTask = func(taskName string, on bool) error {
		if taskName != syncScheduledTaskName {
			t.Fatalf("change task = %q", taskName)
		}
		if !on {
			t.Fatal("expected enable")
		}
		enabled = true
		return nil
	}

	ensureSyncScheduledTask()
	if !enabled {
		t.Fatal("expected scheduled task enable even when already present")
	}
}

func TestEnsureSyncScheduledTaskStaysEnabledWhenAnnouncementsOff(t *testing.T) {
	tmp := t.TempDir()
	utilsDir := filepath.Join(tmp, "utils")
	if err := os.MkdirAll(utilsDir, 0o755); err != nil {
		t.Fatalf("mkdir utils: %v", err)
	}
	syncExe := filepath.Join(utilsDir, "sync.exe")
	if err := os.WriteFile(syncExe, []byte("sync"), 0o644); err != nil {
		t.Fatalf("write sync.exe: %v", err)
	}

	oldProgramRoot := programRootOverride
	oldQuery := queryScheduledTaskXML
	oldCreate := createScheduledTask
	oldChange := changeScheduledTask
	t.Cleanup(func() {
		programRootOverride = oldProgramRoot
		queryScheduledTaskXML = oldQuery
		createScheduledTask = oldCreate
		changeScheduledTask = oldChange
	})

	programRootOverride = tmp
	enabled := false
	queryScheduledTaskXML = func(string) (string, error) {
		return "", errors.New("cannot find the task")
	}
	createScheduledTask = func(string, string) error { return nil }
	changeScheduledTask = func(taskName string, on bool) error {
		if taskName != syncScheduledTaskName {
			t.Fatalf("change task = %q", taskName)
		}
		if !on {
			t.Fatal("license expiry notices require task stay enabled")
		}
		enabled = true
		return nil
	}

	ensureSyncScheduledTask()
	if !enabled {
		t.Fatal("expected scheduled task to stay enabled")
	}
}

func TestEnsureSyncScheduledTaskLogsAndSwallowsErrors(t *testing.T) {
	oldProgramRoot := programRootOverride
	t.Cleanup(func() { programRootOverride = oldProgramRoot })
	programRootOverride = filepath.Join(t.TempDir(), "missing-program-root")

	// Missing sync.exe must not panic or surface as a hard bootstrap failure.
	ensureSyncScheduledTask()
}
