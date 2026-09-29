package legacy

import (
	"strings"
	"testing"
)

func TestFilterSystemPath_KeepsProgramFilesAndNodejs(t *testing.T) {
	in := strings.Join([]string{
		`C:\Windows\system32`,
		`C:\Program Files\Author Software\nvm`,
		`%LOCALAPPDATA%\Author Software\nvm\.nodejs`,
		`C:\Users\corey\AppData\Local\Author Software\nvm`,
		`%NVM_HOME%`,
	}, ";")

	force := map[string]bool{
		strings.ToLower("%NVM_HOME%"): true,
		`c:\old\nvm`:                  true,
	}
	got := FilterSystemPath(in, force)

	wantKeep := []string{
		`C:\Windows\system32`,
		`C:\Program Files\Author Software\nvm`,
		`%LOCALAPPDATA%\Author Software\nvm\.nodejs`,
	}
	wantDrop := []string{
		`C:\Users\corey\AppData\Local\Author Software\nvm`,
		`%NVM_HOME%`,
	}
	for _, s := range wantKeep {
		if !strings.Contains(got, s) {
			t.Fatalf("expected keep %q in %q", s, got)
		}
	}
	for _, s := range wantDrop {
		if strings.Contains(got, s) {
			t.Fatalf("expected drop %q from %q", s, got)
		}
	}
}

func TestFilterSystemPath_ForceRemoveExpandedHome(t *testing.T) {
	in := `C:\Windows;C:\legacy\nvm-home;C:\Program Files\Author Software\nvm`
	force := map[string]bool{normalizePathSeg(`C:\legacy\nvm-home`): true}
	got := FilterSystemPath(in, force)
	if strings.Contains(strings.ToLower(got), `legacy\nvm-home`) {
		t.Fatalf("force remove failed: %q", got)
	}
	if !strings.Contains(got, `Program Files\Author Software\nvm`) {
		t.Fatalf("lost program files: %q", got)
	}
}

func TestIsAuthorNvmProgramRoot(t *testing.T) {
	if !isAuthorNvmProgramRoot(normalizePathSeg(`C:\Users\a\AppData\Local\Author Software\nvm`)) {
		t.Fatal("expected community root drop")
	}
	if isAuthorNvmProgramRoot(normalizePathSeg(`C:\Users\a\AppData\Local\Author Software\nvm\.nodejs`)) {
		t.Fatal(".nodejs must not be treated as program root")
	}
}
