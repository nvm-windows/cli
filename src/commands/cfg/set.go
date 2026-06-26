package cfg

import (
	"common/config"
	"common/notify"
	"common/settings"
	"common/system"
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
			oldValue := currentDisplayValue(key)
			newValue := displayValue(key, value)

			if err := settings.Put(key, value); err != nil {
				log.LogConfigurationChanged(key, newValue, oldValue, log.OutcomeFailed, err.Error())
				errs = append(errs, err.Error())
				continue
			}

			log.LogConfigurationChanged(key, newValue, oldValue, log.OutcomeSucceeded, "")

			enabled := !strings.EqualFold(value, "true")
			if err := setAnnouncements(enabled); err != nil {
				errs = append(errs, err.Error())
			}

			continue
		}

		skip, deferredFn, err := config.Prepare(&key, &value)
		if err != nil {
			outcome := log.OutcomeFailed
			if skip {
				outcome = log.OutcomeSkipped
			}
			log.LogConfigurationChanged(key, displayValue(key, value), currentDisplayValue(key), outcome, err.Error())
			errs = append(errs, err.Error())
		}
		if deferredFn != nil {
			defer deferredFn()
		}
		if skip {
			continue
		}

		oldValue := currentDisplayValue(key)
		newValue := displayValue(key, value)

		if err := settings.Put(key, value); err != nil {
			log.LogConfigurationChanged(key, newValue, oldValue, log.OutcomeFailed, err.Error())
			errs = append(errs, err.Error())
			continue
		}

		log.LogConfigurationChanged(key, newValue, oldValue, log.OutcomeSucceeded, "")
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors updating configuration:\n%s", strings.Join(errs, "\n"))
	}

	for key := range input {
		if key != "mode" {
			log.Logf("%s set to %s", key, displayValue(key, input[key]))
		}

		if err := utility.DisplaySetting(key, "%s set to:\n%s\n"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	var notifyLines []string
	for key, value := range input {
		notifyLines = append(notifyLines, fmt.Sprintf("%s set to %s", key, displayValue(key, value)))
	}
	if len(notifyLines) > 0 && !system.IsAppInForeground() {
		go notify.Send(settings.AppId, "", strings.Join(notifyLines, ", "))
	}

	return nil
}

func currentDisplayValue(key string) string {
	currentValue, err := settings.Get(key)
	if err != nil {
		return ""
	}

	return displayValue(key, currentValue)
}

func displayValue(name string, value interface{}) string {
	masked := settings.MaskedValue(name, value)
	if masked == nil {
		return "(empty)"
	}

	switch v := masked.(type) {
	case []string:
		if len(v) == 0 {
			return "(empty)"
		}
		return strings.Join(v, ",")
	default:
		return fmt.Sprint(v)
	}
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
