package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestListModulesAtScopedNamesUseSlash(t *testing.T) {
	root := t.TempDir()
	mustMkdir := func(parts ...string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(append([]string{root}, parts...)...), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	mustMkdir("left-pad")
	mustMkdir("@opencode", "cli")
	mustMkdir(".bin")

	modules, err := listModulesAt(root)
	if err != nil {
		t.Fatalf("listModulesAt: %v", err)
	}

	if _, ok := modules["left-pad"]; !ok {
		t.Fatalf("missing unscoped module; got %#v", modules)
	}
	if _, ok := modules["@opencode/cli"]; !ok {
		t.Fatalf("missing scoped module with slash name; got %#v", modules)
	}
	if _, ok := modules[filepath.Join("@opencode", "cli")]; ok && runtime.GOOS == "windows" {
		t.Fatalf("scoped module must not use OS separators as npm name; got %#v", modules)
	}
	if _, ok := modules[".bin"]; ok {
		t.Fatalf(".bin must be skipped; got %#v", modules)
	}

	wantPath := filepath.Join(root, "@opencode", "cli")
	if got := modules["@opencode/cli"]; got != wantPath {
		t.Fatalf("scoped path = %q, want %q", got, wantPath)
	}
}
