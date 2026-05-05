package mode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"common/settings"
	"nvm/link"
	"nvm/log"
	"nvm/reshim"
)

func Set(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	exe, _ := os.Executable()
	root := filepath.Dir(exe)
	cfg := settings.Global()

	var value string

	current := cfg.Mode

	switch mode {
	case "link", "symlink":
		if current == "link" {
			return fmt.Errorf("already operating in link mode")
		}

		// Link Mode
		value = "link"
		ensure(root, ".link")
		if err := updateLinkTarget(root); err != nil {
			log.Error(err)
			return err
		}

	case "shim":
		if current == "shim" {
			return fmt.Errorf("already operating in shim mode")
		}

		// Shim mode
		value = "shim"
		ensure(root, ".shim")

		reshim.Run()

	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	// Write the desired mode to the registry.
	if err := settings.Put("mode", value); err != nil {
		err = fmt.Errorf("failed to set mode: %w", err)
		log.Error(err)
		return err
	}

	// Read back the effective mode. settings.Get handles the HKLM policy check transparently.
	actual, err := settings.Get("mode")
	if err != nil {
		err = fmt.Errorf("failed to verify mode: %w", err)
		log.Error(err)
		return err
	}

	// Verify the actual vs chosen to assure policy didn't prevent the change.
	// The registry code will return the current effective value if a policy
	// prevents the change. If they don't match, it was blocked.
	if actualStr, ok := actual.(string); ok && !strings.EqualFold(actualStr, value) {
		err = fmt.Errorf("policy prevents setting mode")
		log.Warn(err.Error())
		return err
	}

	// Reset the internal junction whenever switching modes
	// This applies regardless of enabled status, because the junction determines
	// which Node.js version the system uses
	nodepath := "."
	if mode == "link" {
		nodepath = "nodejs"
	}

	if err := link.NewJunction(filepath.Join(root, "."+value, nodepath), filepath.Join(root, ".nodejs")); err != nil {
		log.Error(err)
		return fmt.Errorf("failed to update internal junction: %w", err)
	}

	log.Logf("switched from %s to %s operating mode", current, value)

	return nil
}

func ensure(root, name string) string {
	path := filepath.Join(root, name)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, 0755)
	}

	if ptr, err := syscall.UTF16PtrFromString(path); err == nil {
		_ = syscall.SetFileAttributes(ptr, syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_DIRECTORY)
	}

	return path
}

// updateLinkTarget creates (or replaces) the .link\nodejs symlink so that it
// points to the currently active Node version's install directory.  If no
// version is active the call is a no-op.
func updateLinkTarget(root string) error {
	activeVersionValue, err := settings.Get("active_version")
	if err != nil {
		return fmt.Errorf("failed to read active version: %w", err)
	}

	activeVersion, ok := activeVersionValue.(string)
	if !ok || strings.TrimSpace(activeVersion) == "" {
		return nil
	}

	cfg := settings.Global()
	installRoot := os.ExpandEnv(cfg.Root)
	versionDir := filepath.Join(installRoot, "v"+activeVersion)
	linkPath := filepath.Join(root, ".link", "nodejs")

	if err := link.Link(versionDir, linkPath); err != nil {
		return fmt.Errorf("failed to create link target %s -> %s: %w", linkPath, versionDir, err)
	}

	return nil
}
