//go:build windows

package bootstrap

import (
	"strings"
	"testing"
)

func TestPsSingleQuote(t *testing.T) {
	if got := psSingleQuote(`C:\a'b`); got != `'C:\a''b'` {
		t.Fatalf("got %q", got)
	}
}

func TestIsMachineProgramRoot(t *testing.T) {
	if !isMachineProgramRoot(`C:\Program Files\Author Software\nvm`) {
		t.Fatal("Program Files should match")
	}
	if !isMachineProgramRoot(`C:\Program Files (x86)\Author Software\nvm`) {
		t.Fatal("Program Files (x86) should match")
	}
	if isMachineProgramRoot(`C:\Users\a\AppData\Local\Author Software\nvm`) {
		t.Fatal("AppData must not match")
	}
}

func TestUnregisterSparseIdentityCommand(t *testing.T) {
	prev := runAppxCommand
	t.Cleanup(func() { runAppxCommand = prev })
	var got string
	runAppxCommand = func(command string) error {
		got = command
		return nil
	}
	if err := UnregisterSparseIdentity(CommunityAppxName); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, CommunityAppxName) || !strings.Contains(got, "Remove-AppxPackage") {
		t.Fatalf("command = %q", got)
	}
}
