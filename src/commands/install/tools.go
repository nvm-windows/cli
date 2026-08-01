package install

import (
	"common/settings"
	"common/verify"
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
	var newestVersionDir string
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

		versionPath := filepath.Join(installRoot, versionDir)
		toolsPath := filepath.Join(versionPath, "install_tools.bat")
		if _, statErr := os.Stat(toolsPath); statErr != nil {
			continue
		}

		if newest == nil || version.GreaterThan(newest) {
			newest = version
			newestVersionDir = versionPath
			newestToolsPath = toolsPath
		}
	}

	if newest == nil {
		return fmt.Errorf("no installed versions with install_tools.bat found")
	}

	expectedToolsPath := filepath.Join(newestVersionDir, "install_tools.bat")
	if !strings.EqualFold(filepath.Clean(newestToolsPath), filepath.Clean(expectedToolsPath)) {
		return fmt.Errorf("refusing to run install_tools.bat: unexpected path %s", newestToolsPath)
	}

	nodeExe := filepath.Join(newestVersionDir, "node.exe")
	if _, err := verify.VerifyNodeExecutable(nodeExe, verify.EffectiveAllowedSigners(cfg.AllowedSigners)); err != nil {
		return fmt.Errorf("refusing to run install_tools.bat: %s failed verification: %w", nodeExe, err)
	}

	cmd := exec.Command("cmd.exe", "/d", "/c", newestToolsPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = newestVersionDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", newestToolsPath, err)
	}

	return nil
}
