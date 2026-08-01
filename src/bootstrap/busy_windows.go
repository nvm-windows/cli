//go:build windows

package bootstrap

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isBusyExecutableError(err error) bool {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if errors.Is(current, windows.ERROR_SHARING_VIOLATION) ||
			errors.Is(current, windows.ERROR_LOCK_VIOLATION) {
			return true
		}
	}
	return false
}
