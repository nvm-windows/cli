package cfg

import (
	"common/config"
	"common/settings"
	"fmt"
	"nvm/commands/cfg/utility"
	"nvm/log"
	"nvm/mode"
	"os"
	"os/exec"
	"strings"
)

type Set struct {
	Pairs []string `arg:"" name:"key=value" help:"One or more configuration assignments in key=value format. Valid keys: ${cfg_opts}"`
}

func (s *Set) Run() error {
	validKeys := make(map[string]struct{}, len(settings.List()))
	for _, key := range settings.List() {
		validKeys[key] = struct{}{}
	}

	input := map[string]string{}
	for _, pair := range s.Pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found || key == "" {
			return fmt.Errorf("invalid assignment %q (expected key=value)", pair)
		}

		if _, ok := validKeys[key]; !ok {
			return fmt.Errorf("invalid configuration key %q", key)
		}

		input[key] = value
	}

	errs := []string{}
	for key, value := range input {
		if key == "mode" {
			// mode changes are handled independently to ensure proper event logging
			if err := mode.Set(value); err != nil {
				errs = append(errs, err.Error())
			} else {
				defer func() {
					fmt.Printf("successfully switched to %s mode\n", value)
				}()
			}
			continue
		}

		if key == "disable_announcements" {
			if err := settings.Put(key, value); err != nil {
				errs = append(errs, err.Error())
				continue
			}

			enabled := !strings.EqualFold(value, "true")
			if err := setAnnouncements(enabled); err != nil {
				errs = append(errs, err.Error())
			}

			continue
		}

		skip, deferredFn, err := config.Prepare(&key, &value)
		if err != nil {
			errs = append(errs, err.Error())
		}
		if deferredFn != nil {
			defer deferredFn()
		}
		if skip {
			continue
		}

		if err := settings.Put(key, value); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors updating configuration:\n%s", strings.Join(errs, "\n"))
	}

	for key := range input {
		if key != "mode" { // mode event logging handled independently
			log.Logf("%s set to %s", key, input[key])
		}

		if err := utility.DisplaySetting(key, "%s set to:\n%s\n"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	return nil
}

func setAnnouncements(enabled bool) error {
	taskNames := []string{"NVM for Windows Sync", "NVM Sync"}
	action := "/Disable"
	if enabled {
		action = "/Enable"
	}

	applied := 0
	cmdErrors := make([]string, 0, len(taskNames))
	for _, taskName := range taskNames {
		cmd := exec.Command("schtasks", "/Change", "/TN", taskName, action)
		output, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(output))
			if msg == "" {
				msg = err.Error()
			}
			cmdErrors = append(cmdErrors, fmt.Sprintf("%s: %s", taskName, msg))
			continue
		}
		applied++
	}

	if applied == 0 {
		return fmt.Errorf("unable to update sync scheduled task state (%s): %s", action, strings.Join(cmdErrors, " | "))
	}

	if enabled {
		log.Log("Project and release announcements have been enabled.")
	} else {
		log.Warn("Project and release announcements have been disabled. To re-enable, run 'nvm config set disable_announcements=false'.")
	}

	if applied > 1 {
		log.Log("Updated multiple sync scheduled task names for compatibility.")
	}
	return nil
}
