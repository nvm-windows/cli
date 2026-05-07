package use

import (
	"common/notify"
	"common/resolver"
	"common/settings"
	"common/system"
	"fmt"
	"nvm/constant"
	"nvm/installer"
	"nvm/link"
	"nvm/log"
	"nvm/prompt"
	"nvm/reshim"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type Version struct {
	constant.FlagInstall
	constant.FlagNoInstall
	constant.ArgVersion
}

func (s *Version) Run() error {
	requestedVersion := s.Version[0]
	cfg := settings.Global()
	autoInstall := cfg.AutoInstall
	if s.Install {
		autoInstall = true
	}
	if s.NoInstall {
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
					ok, err := prompt.Confirm(fmt.Sprintf("Version v%s is not installed. Would you like to install it now?", version), "y")
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
			dots := strings.Count(s.Version[0], ".")
			displayVersion := s.Version[0]
			switch dots {
			case 0:
				displayVersion = s.Version[0] + ".x.x"
			case 1:
				displayVersion = s.Version[0] + ".x"
			case 3:
				return fmt.Errorf("v%s is not a valid version (UnsupportedVersionSpec)", strings.Replace(s.Version[0], "v", "", 1))
			}

			return fmt.Errorf("v%s is not installed", strings.Replace(displayVersion, "v", "", 1))
		}
	}

	var lastVersion string
	if v, err := settings.Get("active_version"); err == nil {
		lastVersion = v.(string)
	}

	if lastVersion == version {
		fmt.Fprintf(os.Stderr, "Already using Node.js v%s\n", version)
		return nil
	}

	if err := settings.Put("active_version", version); err != nil {
		return err
	}

	exe, _ := os.Executable()
	symlink := filepath.Join(filepath.Dir(exe), ".nodejs")

	mode, err := settings.Get("mode")
	if err != nil {
		log.Error(err)
		return err
	} else if mode == "link" {
		source, err := settings.Get("root")
		if err != nil {
			log.Error(err)
			return fmt.Errorf("failed to get install root: %w", err)
		}

		base := filepath.Dir(exe)
		if _, err := os.Lstat(filepath.Join(base, ".link")); err != nil {
			if os.IsNotExist(err) {
				if err := os.Mkdir(filepath.Join(base, ".link"), 0755); err != nil {
					log.Error(err)
					return fmt.Errorf("failed to create .link directory: %w", err)
				} else {
					// Hide the .link directory
					if ptr, err := syscall.UTF16PtrFromString(filepath.Join(base, ".link")); err == nil {
						syscall.SetFileAttributes(ptr, syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_DIRECTORY)
					}
				}
			} else {
				log.Error(err)
				return fmt.Errorf("failed to access .link directory: %w", err)
			}
		}

		if err := link.Link(filepath.Join(source.(string), "v"+version), filepath.Join(base, ".link/nodejs")); err != nil {
			// Don't log to event log here because the link method does this
			// automatically (with more specific detail)
			return err
		}
	}

	// Always update the .nodejs junction to match the current mode
	rel_path := ".shim"
	if mode == "link" {
		rel_path = ".link/nodejs"
	}

	if err := link.Link(filepath.Join(filepath.Dir(exe), rel_path), symlink); err != nil {
		log.Error(err)
		return err
	}

	if mode == "shim" {
		if err := reshim.Run(); err != nil {
			log.Error(err)
			return err
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
