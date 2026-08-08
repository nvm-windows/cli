package log

import "strings"

const (
	OutcomeSucceeded = "Succeeded"
	OutcomeSkipped   = "Skipped"
	OutcomeFailed    = "Failed"
	OutcomeCancelled = "Cancelled"
)

// LogSystemChanged records install/uninstall lifecycle events regardless of
// registry or preference state.
func LogSystemChanged(action, nodeVersion, resolvedPath, outcome, detail string, extras ...StructuredPayload) {
	nodeVersion = strings.TrimSpace(nodeVersion)
	if nodeVersion == "" {
		return
	}

	payload := StructuredPayload{
		"Action":      action,
		"NodeVersion": nodeVersion,
		"Outcome":     outcome,
		"User":        Actor(),
	}
	if strings.TrimSpace(resolvedPath) != "" {
		payload["ResolvedPath"] = resolvedPath
	}
	if strings.TrimSpace(detail) != "" {
		payload["Detail"] = detail
	}
	if len(extras) > 0 {
		for key, value := range extras[0] {
			payload[key] = value
		}
	}

	if strings.EqualFold(outcome, OutcomeFailed) {
		ErrorStructured("nvm.system.changed", payload)
		return
	}

	LogStructured("nvm.system.changed", payload)
}

// LogConfigurationChanged records cfg set/delete outcomes regardless of whether
// prior preference values exist in the registry.
func LogConfigurationChanged(key, value, oldValue, outcome, detail string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}

	payload := StructuredPayload{
		"Action":        "Modified",
		"Configuration": key,
		"Outcome":       outcome,
		"User":          Actor(),
	}
	if strings.TrimSpace(value) != "" {
		payload["Value"] = value
	}
	if strings.TrimSpace(oldValue) != "" {
		payload["Old"] = oldValue
	}
	if strings.TrimSpace(detail) != "" {
		payload["Detail"] = detail
	}

	LogStructured("nvm.configuration.changed", payload)
}
