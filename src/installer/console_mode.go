//go:build windows

package installer

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	enableVirtualTerminalProcessing = 0x0004
)

func enableProgressConsoleMode() error {
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stderr.Fd()} {
		h := windows.Handle(fd)
		var mode uint32
		if err := windows.GetConsoleMode(h, &mode); err != nil {
			continue
		}

		mode |= enableVirtualTerminalProcessing
		_ = windows.SetConsoleMode(h, mode)
	}

	return nil
}
