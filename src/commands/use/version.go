package use

import (
	"common/resolver"
	"common/settings"
	"fmt"
	"nvm/constant"
	"nvm/installer"
	"nvm/log"
	"nvm/prompt"
	"os"
	"strings"
)

type Version struct {
	constant.FlagInstall
	constant.FlagNoInstall
	constant.ArgVersion
	Local bool `flag:"local" short:"l" help:"Use the latest installed version matching the specified partial version."`
}

func getStringSetting(name string) (string, error) {
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

func formatNotInstalledVersion(resolved string) string {
	display := resolver.NormalizeVersion(resolved)
	if strings.Count(display, ".") >= 3 {
		parts := strings.Split(display, ".")
		if len(parts) > 3 {
			return strings.Replace(display, "v", "", 1)
		}
	}
	dots := strings.Count(display, ".")
	switch dots {
	case 0:
		display += ".x.x"
	case 1:
		display += ".x"
	}
	return strings.TrimPrefix(display, "v")
}

func notInstalledUseError(version, mode string, autoInstallDisabled bool) error {
	display := formatNotInstalledVersion(version)
	msg := fmt.Sprintf("v%s is not installed", display)
	if mode == "shim" && autoInstallDisabled {
		msg += "\nEnable auto-install with: nvm config set auto_install=true"
	}
	return fmt.Errorf("%s", msg)
}

func (s *Version) Run() error {
	requestedVersion := s.Version[0]
	cfg := settings.Global()

	// --local: resolve to the best installed match without touching the network.
	if s.Local {
		matched, ok := resolver.LatestInstalledMatch(requestedVersion)
		if !ok {
			return fmt.Errorf("v%s is not installed", formatNotInstalledVersion(requestedVersion))
		}
		requestedVersion = matched
	}

	autoInstall := cfg.AutoInstall
	if s.Install {
		autoInstall = true
	}
	if s.NoInstall {
		autoInstall = false
	}
	// --local always skips installation; the version must already be present.
	if s.Local {
		autoInstall = false
	}

	installed, version, err := resolver.ResolveInstalledVersion(requestedVersion, autoInstall)
	if err != nil {
		return err
	}

	// If the version is not installed, install it
	if !installed {
		shouldInstall := false

		if s.Install {
			shouldInstall = true
		} else if s.NoInstall {
			shouldInstall = false
		} else {
			if cfg.AutoInstall {
				if cfg.AutoInstallPrompt {
					// Check whether a different version in the same major (or major.minor)
					// is already installed, so we can show a more helpful message.
					vparts := strings.SplitN(version, ".", 3)
					// Try the most specific scope first (major.minor), then fall back to major.
					localSpec := vparts[0]
					localMatch, hasLocal := resolver.LatestInstalledMatch(vparts[0] + "." + vparts[1])
					if hasLocal {
						localSpec = vparts[0] + "." + vparts[1]
					} else {
						localMatch, hasLocal = resolver.LatestInstalledMatch(vparts[0])
					}
					if hasLocal && localMatch != version {
						fmt.Fprintf(os.Stderr, "Version v%s is not installed, but v%s is.\n", version, localMatch)
						fmt.Fprintf(os.Stderr, "Run nvm use %s --local to use the latest installed, or specify an exact version.\n", localSpec)
					}
					ok, err := prompt.Confirm(fmt.Sprintf("Would you like to install v%s now?", version), "y")
					if err != nil {
						return fmt.Errorf("failed to read auto-install prompt: %w", err)
					}
					shouldInstall = ok
				} else {
					shouldInstall = true
				}
			}
		}

		if shouldInstall {
			if err := installer.Install(installer.InstallConfig{
				Versions: []string{version},
				// Notify: true,
			}); err != nil {
				log.Error(err)
				return err
			}
		} else {
			if strings.Count(resolver.NormalizeVersion(version), ".") >= 3 &&
				len(strings.Split(resolver.NormalizeVersion(version), ".")) > 3 {
				return fmt.Errorf("v%s is not a valid version (UnsupportedVersionSpec)", formatNotInstalledVersion(version))
			}
			mode, modeErr := getStringSetting("mode")
			if modeErr != nil {
				return modeErr
			}
			return notInstalledUseError(version, mode, !cfg.AutoInstall && !s.NoInstall)
		}
	}

	var lastVersion string
	if v, err := getStringSetting("active_version"); err == nil {
		lastVersion = v
	}

	if lastVersion == version {
		fmt.Fprintf(os.Stderr, "Already using Node.js v%s\n", version)
		return nil
	}

	return installer.ActivateVersion(version)
}
