package log

import (
	"common/eventlog"
	"common/license"
	"encoding/json"
	"os"
	"os/user"
	"strings"
)

// StructuredPayload is a convenience alias for structured event fields.
// Callers can pass any JSON-marshalable value to LogStructured/WarnStructured/ErrorStructured.
type StructuredPayload map[string]any

func RegisterEventSource(appName string) error {
	return eventlog.RegisterEventSource(appName)
}

func UnregisterEventSource(appName string) error {
	return eventlog.UnregisterEventSource(appName)
}

func NewEventLogger(appName ...string) (*eventlog.EventLogger, error) {
	return eventlog.NewEventLogger(appName...)
}

func Log(message string, code ...int) {
	eventlog.Log(message, code...)
}

func Logf(format string, args ...interface{}) {
	eventlog.Logf(format, args...)
}

func LogStructured(eventName string, payload any, code ...int) {
	if !license.AllowsStructuredLogging() {
		Log(formatStructuredFallback(eventName, payload), code...)
		return
	}
	eventlog.LogStructured(eventName, payload, code...)
}

func Warn(message string, code ...int) {
	eventlog.Warn(message, code...)
}

func Warnf(format string, args ...interface{}) {
	eventlog.Warnf(format, args...)
}

func WarnStructured(eventName string, payload any, code ...int) {
	if !license.AllowsStructuredLogging() {
		Warn(formatStructuredFallback(eventName, payload), code...)
		return
	}
	eventlog.WarnStructured(eventName, payload, code...)
}

func Error(err error, code ...int) {
	eventlog.Error(err, code...)
}

func Errorf(format string, args ...interface{}) {
	eventlog.Errorf(format, args...)
}

func ErrorStructured(eventName string, payload any, code ...int) {
	if !license.AllowsStructuredLogging() {
		Error(fmtError(formatStructuredFallback(eventName, payload)), code...)
		return
	}
	eventlog.ErrorStructured(eventName, payload, code...)
}

type fallbackError string

func (e fallbackError) Error() string { return string(e) }

func fmtError(message string) error {
	return fallbackError(message)
}

func formatStructuredFallback(eventName string, payload any) string {
	eventName = strings.TrimSpace(eventName)
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) == 0 || string(encoded) == "null" || string(encoded) == "{}" {
		return eventName
	}
	if eventName == "" {
		return string(encoded)
	}
	return eventName + " " + string(encoded)
}

// Actor returns a stable best-effort user identifier for audit events.
func Actor() string {
	if current, err := user.Current(); err == nil {
		name := strings.TrimSpace(current.Username)
		if name != "" {
			return name
		}
	}

	domain := strings.TrimSpace(os.Getenv("USERDOMAIN"))
	username := strings.TrimSpace(os.Getenv("USERNAME"))
	if domain != "" && username != "" {
		return domain + `\\` + username
	}
	if username != "" {
		return username
	}

	return "unknown"
}

// ExampleStructuredUsage demonstrates how to send a custom structured event.
// Keep this as an inline reference while structured event adoption rolls out.
func ExampleStructuredUsage() {
	LogStructured("node.install.started", StructuredPayload{
		"version":       "24.0.0",
		"requestedBy":   "cli",
		"operatingMode": "link",
	}, 4101)
}
