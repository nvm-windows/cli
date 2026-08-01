package clitest

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SeedVersionOptions configures SeedVersion.
type SeedVersionOptions struct {
	// NodeSource copies this file to node.exe when set.
	NodeSource string
}

// SeedVersion creates a minimal installed version directory under InstallRoot.
func (s *Sandbox) SeedVersion(version string, opts *SeedVersionOptions) string {
	s.t.Helper()

	version = normalizeVersion(version)
	versionDir := filepath.Join(s.InstallRoot, version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		s.t.Fatalf("SeedVersion(%q) MkdirAll error = %v", version, err)
	}

	nodePath := filepath.Join(versionDir, "node.exe")
	source := ""
	if opts != nil {
		source = strings.TrimSpace(opts.NodeSource)
	}
	if source == "" {
		source = strings.TrimSpace(os.Getenv("NVM_TEST_SIGNED_NODE"))
	}

	if source != "" {
		if err := copyFile(source, nodePath); err != nil {
			s.t.Fatalf("SeedVersion(%q) copy node.exe error = %v", version, err)
		}
		s.seedNpmLayout(versionDir)
		return nodePath
	}

	if err := os.WriteFile(nodePath, []byte("nvm clitest node stub"), 0o644); err != nil {
		s.t.Fatalf("SeedVersion(%q) WriteFile error = %v", version, err)
	}

	s.seedNpmLayout(versionDir)
	return nodePath
}

func (s *Sandbox) seedNpmLayout(versionDir string) {
	s.t.Helper()

	npmDir := filepath.Join(versionDir, "node_modules", "npm")
	if err := os.MkdirAll(npmDir, 0o755); err != nil {
		s.t.Fatalf("seedNpmLayout MkdirAll error = %v", err)
	}
	pkg := filepath.Join(npmDir, "package.json")
	if err := os.WriteFile(pkg, []byte(`{"version":"10.5.0"}`), 0o644); err != nil {
		s.t.Fatalf("seedNpmLayout WriteFile error = %v", err)
	}
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		return "v0.0.0"
	}
	return "v" + version
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
