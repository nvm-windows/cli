package clitest

import (
	"os"
)

// Output holds captured process streams from a command invocation.
type Output struct {
	Stdout string
	Stderr string
}

// CaptureOutput runs fn with os.Stdout and os.Stderr redirected.
// Temp files are used instead of pipes so commands that call os.Stdout.Sync()
// (for example list --json) do not hang on Windows.
func CaptureOutput(fn func() error) (out Output, err error) {
	stdoutFile, err := os.CreateTemp("", "clitest-stdout-*")
	if err != nil {
		return out, err
	}
	stdoutPath := stdoutFile.Name()

	stderrFile, err := os.CreateTemp("", "clitest-stderr-*")
	if err != nil {
		stdoutFile.Close()
		os.Remove(stdoutPath)
		return out, err
	}
	stderrPath := stderrFile.Name()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = stdoutFile
	os.Stderr = stderrFile

	runErr := fn()

	os.Stdout = oldStdout
	os.Stderr = oldStderr
	stdoutFile.Close()
	stderrFile.Close()

	defer os.Remove(stdoutPath)
	defer os.Remove(stderrPath)

	stdoutBytes, readErr := os.ReadFile(stdoutPath)
	if readErr != nil {
		return out, readErr
	}
	stderrBytes, readErr := os.ReadFile(stderrPath)
	if readErr != nil {
		return out, readErr
	}

	out.Stdout = string(stdoutBytes)
	out.Stderr = string(stderrBytes)
	return out, runErr
}
