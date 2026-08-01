//go:build !windows

package bootstrap

func isBusyExecutableError(err error) bool {
	return false
}
