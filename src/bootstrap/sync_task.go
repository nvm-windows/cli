package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"common/settings"
	"nvm/log"
)

const syncScheduledTaskName = "NVM for Windows Sync"

var (
	createScheduledTask = defaultScheduledTaskCreate
	changeScheduledTask = defaultScheduledTaskChange
)

// ensureSyncScheduledTask registers the per-user hourly sync task used for
// news/release announcements. Errors are logged and swallowed so bootstrap
// never fails solely because Task Scheduler is unavailable.
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

	disabled, err := announcementsDisabled()
	if err != nil {
		return fmt.Errorf("read disable_announcements: %w", err)
	}
	if disabled {
		if err := changeScheduledTask(syncScheduledTaskName, false); err != nil {
			return fmt.Errorf("disable scheduled task %q: %w", syncScheduledTaskName, err)
		}
	}

	return nil
}

func announcementsDisabled() (bool, error) {
	value, err := settings.Get("disable_announcements")
	if err != nil {
		return false, err
	}
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true") || strings.TrimSpace(v) == "1", nil
	default:
		return false, nil
	}
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
