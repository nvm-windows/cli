package install

import (
	"common/settings"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

type Tools struct{}

func (c *Tools) Run() error {
	cfg := settings.Global()

	if !cfg.AllowToolInstall {
		return fmt.Errorf("installation of native tools is blocked by this computer's policy")
	}

	installRoot := settings.Expand(cfg.Root)

	entries, err := os.ReadDir(installRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no versions installed")
		}
		return err
	}

	var newest *semver.Version
	var newestToolsPath string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		versionDir := entry.Name()
		if !strings.HasPrefix(strings.ToLower(versionDir), "v") {
			continue
		}

		versionValue := strings.TrimPrefix(versionDir, "v")
		version, parseErr := semver.NewVersion(versionValue)
		if parseErr != nil {
			continue
		}

		toolsPath := filepath.Join(installRoot, versionDir, "install_tools.bat")
		if _, statErr := os.Stat(toolsPath); statErr != nil {
			continue
		}

		if newest == nil || version.GreaterThan(newest) {
			newest = version
			newestToolsPath = toolsPath
		}
	}

	if newest == nil {
		return fmt.Errorf("no installed versions with install_tools.bat found")
	}

	cmd := exec.Command("cmd.exe", "/d", "/c", newestToolsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = filepath.Dir(newestToolsPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", newestToolsPath, err)
	}

	return nil
}
