package installer

import (
	"os"
	"path/filepath"
)

// RegisterInstalledVersions ensures all installed Node.js versions are
// represented in Windows Apps uninstall metadata.
func RegisterInstalledVersions() error {
	versions := scanInstalledVersions()
	for _, version := range versions {
		installDir := getRoot(version)
		nodeExe := filepath.Join(installDir, "node.exe")
		if _, err := os.Stat(nodeExe); err != nil {
			continue
		}

		registerNodeVersion(version, installDir, "")
	}

	return nil
}
