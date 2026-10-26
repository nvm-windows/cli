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

func TestEnsureUserNodejsPath_PrependsExpandedAndDropsUnexpanded(t *testing.T) {
	root := `C:\Users\a\AppData\Local\Author Software\nvm`
	nodejs := filepath.Join(root, ".nodejs")
	in := strings.Join([]string{
		unexpandedUserNodejsPath,
		`C:\Windows\system32`,
		`C:\Tools`,
	}, ";")

	got, changed := ensureUserNodejsPath(in, nodejs)
	if !changed {
		t.Fatal("want PATH rewrite")
	}
	if !pathHasSegment(got, nodejs) {
		t.Fatalf("expected expanded %q in %q", nodejs, got)
	}
	if pathHasSegment(got, unexpandedUserNodejsPath) {
		t.Fatalf("unexpanded %%LOCALAPPDATA%% .nodejs still in %q", got)
	}
	parts := strings.Split(got, ";")
	if normalizePathMatch(parts[0]) != normalizePathMatch(nodejs) {
		t.Fatalf("expanded .nodejs should be first, got %q", got)
	}
}

func TestEnsureUserNodejsPath_IdempotentWhenExpandedPresent(t *testing.T) {
	root := `C:\Users\a\AppData\Local\Author Software\nvm`
	nodejs := filepath.Join(root, ".nodejs")
	in := strings.Join([]string{nodejs, `C:\Windows\system32`}, ";")
	got, changed := ensureUserNodejsPath(in, nodejs)
	if changed {
		t.Fatalf("PATH already expanded, got rewrite %q", got)
	}
	if got != in {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestCanonicalUserDataRoot_UsesLocalAppDataOnly(t *testing.T) {
	got, ok := canonicalUserDataRootFrom(func(name string) (string, bool) {
		if name == "LOCALAPPDATA" {
			return `C:\Users\a\AppData\Local`, true
		}
		return "", false
	})
	if !ok {
		t.Fatal("want LocalAppData data root")
	}
	want := filepath.Join(`C:\Users\a\AppData\Local`, "Author Software", "nvm")
	if normalizePathMatch(got) != normalizePathMatch(want) {
		t.Fatalf("got %q want %q", got, want)
	}

	_, ok = canonicalUserDataRootFrom(func(name string) (string, bool) {
		if name == "LOCALAPPDATA" {
			return `C:\Windows\ServiceProfiles\LocalService\AppData\Local`, true
		}
		return "", false
	})
	if ok {
		t.Fatal("service profile LocalAppData must not be a data root")
	}

	_, ok = canonicalUserDataRootFrom(func(string) (string, bool) {
		return "", false
	})
	if ok {
		t.Fatal("empty LOCALAPPDATA must not yield a data root")
	}
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
