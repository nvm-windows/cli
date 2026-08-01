package clitest

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestdataPath returns a path under test/testdata.
func TestdataPath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", name)
}
