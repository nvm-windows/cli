package installer

import (
	"common/settings"
	"common/system"
	"common/verifycache"
	"fmt"
	"nvm/bootstrap"
	"nvm/link"
	"nvm/log"
	nvmreshim "nvm/reshim"
	"os"
	"path/filepath"
	"strings"

	"common/notify"

	"github.com/Masterminds/semver/v3"
)

// ActivateVersion sets the default Node.js version and updates runtime links/shims.
func ActivateVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("version must not be empty")
	}

	lastVersion, err := stringSetting("active_version")
	if err != nil {
		return err
	}

	if lastVersion == version {
		fmt.Fprintf(os.Stderr, "Already using Node.js v%s\n", version)
		return nil
	}

	if err := settings.Put("active_version", version); err != nil {
		return err
	}

	base, err := bootstrap.DataRoot()
	if err != nil {
		log.Error(err)
		return fmt.Errorf("failed to resolve runtime root: %w", err)
	}
	symlink := filepath.Join(base, ".nodejs")

	mode, err := stringSetting("mode")
	if err != nil {
		log.Error(err)
		return err
	}

	if mode == "link" {
		source, err := stringSetting("root")
		if err != nil {
			log.Error(err)
			return fmt.Errorf("failed to get install root: %w", err)
		}

		if _, err := os.Lstat(filepath.Join(base, ".link")); err != nil {
			if os.IsNotExist(err) {
				if err := bootstrap.EnsureHiddenDir(filepath.Join(base, ".link")); err != nil {
					log.Error(err)
					return fmt.Errorf("failed to create .link directory: %w", err)
				}
			} else {
				log.Error(err)
				return fmt.Errorf("failed to access .link directory: %w", err)
			}
		}

		versionDir := filepath.Join(source, "v"+version)
		if err := bootstrap.ValidateVersionActivation(versionDir); err != nil {
			return err
		}
		if err := link.Link(versionDir, filepath.Join(base, ".link/nodejs")); err != nil {
			return err
		}
	}

	relPath := ".shim"
	if mode == "link" {
		relPath = ".link/nodejs"
	}

	if err := link.Link(filepath.Join(base, relPath), symlink); err != nil {
		log.Error(err)
		return err
	}

	if mode == "shim" {
		if err := nvmreshim.Run(); err != nil {
			log.Error(err)
			return err
		}
	} else if source, err := stringSetting("root"); err == nil {
		versionDir := filepath.Join(source, "v"+version)
		if err := verifycache.SignNodeCache(filepath.Join(versionDir, "node.exe")); err != nil {
			log.Logf("verify cache warning: %v", err)
		}
		if err := verifycache.SignVersionScripts(versionDir); err != nil {
			log.Logf("script trust warning: %v", err)
		}
	}

	if err := settings.Put("last_version", lastVersion); err != nil {
		log.Error(err)
		return err
	}

	log.Logf("Now using Node.js v%s by default", version)
	msg := fmt.Sprintf("Now using Node.js v%s by default.", version)
	fmt.Fprintln(os.Stdout, msg)
	if !system.IsAppInForeground() {
		go notify.Send(settings.AppId, "", msg)
	}

	return nil
}

func activateDefaultIfFirstInstall(hadInstalledVersions bool, installedVersions []string) error {
	if hadInstalledVersions {
		return nil
	}

	version := highestSemverVersion(installedVersions)
	if version == "" {
		return nil
	}

	if strings.TrimSpace(settings.Global().ActiveVersion) != "" {
		return nil
	}

	return ActivateVersion(version)
}

func highestSemverVersion(versions []string) string {
	var best *semver.Version
	var bestLabel string

	for _, raw := range versions {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}

		parsed, err := semver.NewVersion(label)
		if err != nil {
			continue
		}

		if best == nil || parsed.GreaterThan(best) {
			best = parsed
			bestLabel = label
		}
	}

	return bestLabel
}

func stringSetting(name string) (string, error) {
	value, err := settings.Get(name)
	if err != nil {
		return "", err
	}

	text, ok := value.(string)
	if !ok {
		return "", nil
	}

	return text, nil
}
