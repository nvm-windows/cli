package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"nvm/log"
)

const syncScheduledTaskName = "NVM for Windows Sync"

var (
	createScheduledTask = defaultScheduledTaskCreate
	changeScheduledTask = defaultScheduledTaskChange
)

// ensureSyncScheduledTask registers the per-user hourly sync task used for
// news/release announcements and certified license expiry toasts. Errors are
// logged and swallowed so bootstrap never fails solely because Task Scheduler
// is unavailable.
func ensureSyncScheduledTask() {
	if err := ensureSyncScheduledTaskErr(); err != nil {
		log.Warnf("sync scheduled task setup skipped: %v", err)
	}
}

func ensureSyncScheduledTaskErr() error {
	syncExe, err := UtilityPath("sync.exe")
	if err != nil {
		return err
	}

	if _, err := os.Stat(syncExe); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("sync.exe not found at %s", syncExe)
		}
		return fmt.Errorf("inspect sync.exe: %w", err)
	}

	xml, queryErr := queryScheduledTaskXML(syncScheduledTaskName)
	taskMissing := queryErr != nil && isScheduledTaskMissing(queryErr, xml)
	if queryErr != nil && !taskMissing {
		return fmt.Errorf("query scheduled task %q: %w", syncScheduledTaskName, queryErr)
	}

	needsCreate := taskMissing || !valueReferencesPath(xml, syncExe)
	if needsCreate {
		if err := createScheduledTask(syncScheduledTaskName, syncExe); err != nil {
			return fmt.Errorf("create scheduled task %q: %w", syncScheduledTaskName, err)
		}
		log.Logf("Registered scheduled task %q -> %s --background sync", syncScheduledTaskName, syncExe)
	}

	// Always leave the task enabled. disable_announcements only skips news/releases
	// inside sync.exe; license expiry toasts still need the hourly run.
	if err := changeScheduledTask(syncScheduledTaskName, true); err != nil {
		return fmt.Errorf("enable scheduled task %q: %w", syncScheduledTaskName, err)
	}

	return nil
}

func defaultScheduledTaskCreate(taskName, syncExe string) error {
	// Match community Inno: hourly task as current user, no /RU SYSTEM.
	tr := fmt.Sprintf(`"%s" --background sync`, syncExe)
	output, err := exec.Command(
		"schtasks",
		"/Create",
		"/SC", "HOURLY",
		"/MO", "1",
		"/TN", taskName,
		"/TR", tr,
		"/F",
	).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, trimmed)
	}
	return nil
}

func defaultScheduledTaskChange(taskName string, enabled bool) error {
	action := "/Disable"
	if enabled {
		action = "/Enable"
	}
	output, err := exec.Command("schtasks", "/Change", "/TN", taskName, action).CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, trimmed)
	}
	return nil
}
