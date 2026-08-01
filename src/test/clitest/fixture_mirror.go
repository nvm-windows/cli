package clitest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"nvm/commands/cache"
)

// ReloadCacheStore updates cache command paths for the active sandbox profile.
func ReloadCacheStore() {
	cache.ReloadStore()
}

// SeedCacheArchive creates a fake cached Node.js archive under the sandbox cache root.
func (s *Sandbox) SeedCacheArchive(version string) string {
	s.t.Helper()

	ReloadCacheStore()
	versionsDir := filepath.Join(s.DataRoot(), ".cache", "versions")
	if err := os.MkdirAll(versionsDir, 0o755); err != nil {
		s.t.Fatalf("SeedCacheArchive(%q) MkdirAll error = %v", version, err)
	}

	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	name := fmt.Sprintf("node-v%s-win-%s.7z", version, arch)
	path := filepath.Join(versionsDir, name)
	if err := os.WriteFile(path, []byte("nvm clitest cache archive"), 0o644); err != nil {
		s.t.Fatalf("SeedCacheArchive(%q) WriteFile error = %v", version, err)
	}

	ReloadCacheStore()
	return path
}

// SeedCacheMetadata creates a file in the metadata cache directory.
func (s *Sandbox) SeedCacheMetadata(name string, content []byte) string {
	s.t.Helper()

	ReloadCacheStore()
	metadataDir := filepath.Join(s.DataRoot(), ".cache", "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		s.t.Fatalf("SeedCacheMetadata(%q) MkdirAll error = %v", name, err)
	}

	path := filepath.Join(metadataDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		s.t.Fatalf("SeedCacheMetadata(%q) WriteFile error = %v", name, err)
	}

	ReloadCacheStore()
	return path
}

// StartNodeMirrorFixture serves a static index.tab for offline resolver/list tests.
func StartNodeMirrorFixture(t *testing.T, tabPath string) *httptest.Server {
	t.Helper()

	tab, err := os.ReadFile(tabPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", tabPath, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/index.tab") {
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = w.Write(tab)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ApplyNodeMirrorFixture points node_mirror at a local test server.
func (s *Sandbox) ApplyNodeMirrorFixture(t *testing.T, tabPath string) *httptest.Server {
	s.t.Helper()

	srv := StartNodeMirrorFixture(t, tabPath)
	if err := s.WriteSetting("node_mirror", srv.URL); err != nil {
		t.Fatalf("WriteSetting(node_mirror) error = %v", err)
	}
	return srv
}
