package mode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"common/settings"
	"nvm/bootstrap"
	"nvm/link"
	"nvm/log"
	"nvm/reshim"
)

func Set(mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	cfg := settings.Global()
	current := cfg.Mode

	root, err := bootstrap.DataRoot()
	if err != nil {
		err = fmt.Errorf("failed to resolve runtime root: %w", err)
		logConfigurationOutcome("mode", mode, current, log.OutcomeFailed, err.Error())
		return err
	}

	var value string

	switch mode {
	case "link", "symlink":
		if current == "link" {
			err := fmt.Errorf("already operating in link mode")
			logConfigurationOutcome("mode", "link", current, log.OutcomeSkipped, err.Error())
			return err
		}

		// Link Mode
		value = "link"
		ensure(root, ".link")
		if err := updateLinkTarget(root); err != nil {
			logConfigurationOutcome("mode", "link", current, log.OutcomeFailed, err.Error())
			return err
		}

	case "shim":
		if current == "shim" {
			err := fmt.Errorf("already operating in shim mode")
			logConfigurationOutcome("mode", "shim", current, log.OutcomeSkipped, err.Error())
			return err
		}

		// Shim mode
		value = "shim"
		ensure(root, ".shim")

		reshim.Run()

	default:
		err := fmt.Errorf("invalid mode: %s", mode)
		logConfigurationOutcome("mode", mode, current, log.OutcomeFailed, err.Error())
		return err
	}

	// Write the desired mode to the registry.
	if err := settings.Put("mode", value); err != nil {
		err = fmt.Errorf("failed to set mode: %w", err)
		logConfigurationOutcome("mode", value, current, log.OutcomeFailed, err.Error())
		return err
	}

	// Read back the effective mode. settings.Get handles the HKLM policy check transparently.
	actual, err := settings.Get("mode")
	if err != nil {
		err = fmt.Errorf("failed to verify mode: %w", err)
		logConfigurationOutcome("mode", value, current, log.OutcomeFailed, err.Error())
		return err
	}

	// Verify the actual vs chosen to assure policy didn't prevent the change.
	// The registry code will return the current effective value if a policy
	// prevents the change. If they don't match, it was blocked.
	if actualStr, ok := actual.(string); ok && !strings.EqualFold(actualStr, value) {
		err := fmt.Errorf("policy prevents setting mode")
		logConfigurationOutcome("mode", value, current, log.OutcomeFailed, err.Error())
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
		err = fmt.Errorf("failed to update internal junction: %w", err)
		logConfigurationOutcome("mode", value, current, log.OutcomeFailed, err.Error())
		return err
	}

	logConfigurationOutcome("mode", value, current, log.OutcomeSucceeded, "")
	log.Logf("switched from %s to %s operating mode", current, value)

	return nil
}

func logConfigurationOutcome(key, value, oldValue, outcome, detail string) {
	log.LogConfigurationChanged(key, strings.TrimSpace(value), strings.TrimSpace(oldValue), outcome, detail)
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
