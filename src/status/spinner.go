package status

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

type Position int

const (
	SPINNER_BEFORE Position = iota
	SPINNER_AFTER
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type Spinner struct {
	Label    string
	frames   []string
	Duration time.Duration
	close    func()
	Position Position
}

func NewSpinner(label string, frames ...[]string) *Spinner {
	// f := []string{"|", "/", "-", "\\"}
	f := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if len(frames) > 0 {
		f = frames[0]
	}

	return &Spinner{
		Label:    label,
		frames:   f,
		Duration: time.Duration(100 * time.Millisecond),
		Position: SPINNER_BEFORE,
	}
}

func (s *Spinner) Start() {
	// Prevent starting again
	if s.close != nil {
		return
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(stopped)

		ticker := time.NewTicker(s.Duration)
		defer ticker.Stop()

		i := 0
		for {
			width := getWidth(syscall.STD_ERROR_HANDLE)
			first := s.frames[i%len(s.frames)]
			second := s.Label
			if s.Position == SPINNER_AFTER {
				first, second = second, first
			}

			labelWidth := utf8.RuneCountInString(s.Label)
			padWidth := width - labelWidth - 2
			if padWidth < 0 {
				padWidth = 0
			}

			fmt.Fprintf(os.Stderr, "\r%s %s%s", first, second, strings.Repeat(" ", padWidth))
			i++

			select {
			case <-done:
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", width))
				return
			case <-ticker.C:
			}
		}
	}()

	s.close = func() {
		once.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func (s *Spinner) Stop() {
	if s.close != nil {
		s.close()
		s.close = nil
	}
}

// PrintBefore temporarily stops the spinner, prints a line to stdout,
// then restarts the spinner if it was running.
func (s *Spinner) PrintBefore(text string) {
	wasRunning := s.close != nil
	if wasRunning {
		s.Stop()
	}

	fmt.Fprintln(os.Stdout, text)

	if wasRunning {
		s.Start()
	}
}

// PrintBeforef temporarily stops the spinner, prints a formatted line to
// stdout, then restarts the spinner if it was running.
func (s *Spinner) PrintBeforef(format string, args ...interface{}) {
	s.PrintBefore(fmt.Sprintf(format, args...))
}

type coord struct{ X, Y int16 }
type smallRect struct{ Left, Top, Right, Bottom int16 }
type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func getWidth(stdHandle int) int {
	var info consoleScreenBufferInfo
	handle, _ := syscall.GetStdHandle(stdHandle)

	// Windows calls generally return 0 on failure
	ret, _, _ := procGetConsoleScreenBufferInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&info)),
	)
	if ret == 0 {
		return 80
	}
	return int(info.Window.Right - info.Window.Left + 1)
}
