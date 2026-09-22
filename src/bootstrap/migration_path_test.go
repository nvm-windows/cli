package bootstrap

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterUserPath_KeepsNodejsDropsProgramRoot(t *testing.T) {
	root := `C:\Users\a\AppData\Local\Author Software\nvm`
	nodejs := filepath.Join(root, ".nodejs")
	in := strings.Join([]string{
		`C:\Windows\system32`,
		root,
		nodejs,
		`C:\Tools`,
		`%NVM_HOME%`,
	}, ";")
	force := map[string]bool{
		strings.ToLower("%NVM_HOME%"): true,
		normalizePathMatch(root):      true,
	}

	got := filterUserPath(in, root, force)

	for _, keep := range []string{`C:\Windows\system32`, nodejs, `C:\Tools`} {
		if !pathHasSegment(got, keep) {
			t.Fatalf("expected keep %q in %q", keep, got)
		}
	}
	for _, drop := range []string{root, `%NVM_HOME%`} {
		if pathHasSegment(got, drop) {
			t.Fatalf("expected drop %q from %q", drop, got)
		}
	}
}

func pathHasSegment(path, segment string) bool {
	want := normalizePathMatch(segment)
	for _, part := range strings.Split(path, ";") {
		if normalizePathMatch(part) == want {
			return true
		}
	}
	return false
}

func TestLooksLikeLegacyNvmSymlink(t *testing.T) {
	if !looksLikeLegacyNvmSymlink(`C:\nodejs`) {
		t.Fatal("want classic C:\\nodejs")
	}
	if !looksLikeLegacyNvmSymlink(`C:\Users\a\AppData\Local\Author Software\nvm\.nodejs`) {
		t.Fatal("want Author Software .nodejs")
	}
	if looksLikeLegacyNvmSymlink(`D:\custom\node-link`) {
		t.Fatal("custom link must not match")
	}
}
