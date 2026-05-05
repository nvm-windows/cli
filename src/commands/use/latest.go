package use

import (
	"common/settings"
	"fmt"
	"nvm/log"
	"os"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

type LatestAlias struct{}

func (c *LatestAlias) Run() error {
	newest, err := resolveLatestInstalledSemver()
	if err != nil {
		log.Error(err)
		fmt.Fprint(os.Stderr, err.Error())
		return err
	}

	cmd := Version{}
	cmd.Version = []string{newest}
	return cmd.Run()
}

func resolveLatestInstalledSemver() (string, error) {
	cfg := settings.Global()
	installRoot := settings.Expand(cfg.Root)

	entries, err := os.ReadDir(installRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no versions installed")
		}
		return "", err
	}

	var newest *semver.Version
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		versionDir := entry.Name()
		if !strings.HasPrefix(strings.ToLower(versionDir), "v") {
			continue
		}

		versionValue := strings.TrimPrefix(versionDir, "v")
		v, parseErr := semver.NewVersion(versionValue)
		if parseErr != nil {
			continue
		}

		nodeExe := filepath.Join(installRoot, versionDir, "node.exe")
		if _, statErr := os.Stat(nodeExe); statErr != nil {
			continue
		}

		if newest == nil || v.GreaterThan(newest) {
			newest = v
		}
	}

	if newest == nil {
		return "", fmt.Errorf("no installed semver versions found")
	}

	return newest.String(), nil
}
