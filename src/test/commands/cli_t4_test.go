package commands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nvm/test/clitest"
)

func TestCLICacheViewListsSeededArchive(t *testing.T) {
	sb := clitest.NewSandbox(t)
	archiveName := filepath.Base(sb.SeedCacheArchive("22.0.0"))

	stdout, stderr, err := sb.Execute("cache", "view", "versions")
	if err != nil {
		t.Fatalf("Execute(cache view versions) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, archiveName) {
		t.Fatalf("Execute(cache view versions) stdout = %q, want %q", stdout, archiveName)
	}
}

func TestCLICacheViewJSON(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedCacheArchive("22.0.0")

	stdout, stderr, err := sb.Execute("cache", "view", "--json")
	if err != nil {
		t.Fatalf("Execute(cache view --json) error = %v stderr = %q", err, stderr)
	}

	var out map[string]struct {
		TotalFiles int `json:"total_files"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v stdout = %q", err, stdout)
	}
	versions, ok := out["versions"]
	if !ok || versions.TotalFiles < 1 {
		t.Fatalf("cache view JSON = %+v, want versions.total_files >= 1", out)
	}
}

func TestCLICacheRemoveAll(t *testing.T) {
	sb := clitest.NewSandbox(t)
	archivePath := sb.SeedCacheArchive("22.0.0")
	sb.SeedCacheMetadata("fixture.txt", []byte("metadata"))

	stdout, stderr, err := sb.Execute("cache", "remove", "all")
	if err != nil {
		t.Fatalf("Execute(cache remove all) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "cleared versions cache") {
		t.Fatalf("Execute(cache remove all) stdout = %q, want cleared message", stdout)
	}
	if _, statErr := os.Stat(archivePath); !os.IsNotExist(statErr) {
		t.Fatalf("cache archive still exists after remove all: %v", statErr)
	}
}

func TestCLIListReleasesUsesMirrorFixture(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))

	stdout, stderr, err := sb.Execute("list", "releases", "--limit", "2")
	if err != nil {
		t.Fatalf("Execute(list releases) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(list releases) stdout = %q, want fixture version", stdout)
	}
}

func TestCLIListReleasesJSON(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.ApplyNodeMirrorFixture(t, clitest.TestdataPath(t, "index.tab"))

	stdout, stderr, err := sb.Execute("list", "releases", "--json", "--no-limit")
	if err != nil {
		t.Fatalf("Execute(list releases --json) error = %v stderr = %q", err, stderr)
	}

	var out []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v stdout = %q", err, stdout)
	}
	if len(out) == 0 {
		t.Fatalf("list releases JSON len = 0, want >= 1")
	}
}

func TestCLIListCachedEmpty(t *testing.T) {
	sb := clitest.NewSandbox(t)

	stdout, stderr, err := sb.Execute("list", "cached")
	if err != nil {
		t.Fatalf("Execute(list cached) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "No cached versions") {
		t.Fatalf("Execute(list cached) stdout = %q, want empty cached message", stdout)
	}
}

func TestCLIListCachedSeeded(t *testing.T) {
	sb := clitest.NewSandbox(t)
	sb.SeedCacheArchive("22.0.0")

	stdout, stderr, err := sb.Execute("list", "cached")
	if err != nil {
		t.Fatalf("Execute(list cached) error = %v stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "22.0.0") {
		t.Fatalf("Execute(list cached) stdout = %q, want cached version", stdout)
	}
}
