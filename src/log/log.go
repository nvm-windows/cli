package log

import "common/eventlog"

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

func Warn(message string, code ...int) {
	eventlog.Warn(message, code...)
}

func Warnf(format string, args ...interface{}) {
	eventlog.Warnf(format, args...)
}

func Error(err error, code ...int) {
	eventlog.Error(err, code...)
}

func Errorf(format string, args ...interface{}) {
	eventlog.Errorf(format, args...)
}
