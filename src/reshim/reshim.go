package reshim

import (
	"common/settings"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

func Run() error {
	installRoot := expand(settings.Global().Root)
	appRoot := filepath.Clean(filepath.Join(installRoot, ".."))
	reshimPath := filepath.Clean(filepath.Join(appRoot, "utils", "reshim.exe"))

	if _, err := os.Stat(reshimPath); err != nil {
		return fmt.Errorf("reshim not found at %s: %w", reshimPath, err)
	}

	// Run asynchronously so nvm use returns immediately.
	cmd := exec.Command(reshimPath)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("reshim failed to start: %w", err)
	}
	// Detach: do not call cmd.Wait(). Reshim runs in the background.

	return nil
}

func expand(path string) string {
	re := regexp.MustCompile(`%([^%]+)%`)
	return re.ReplaceAllStringFunc(path, func(match string) string {
		varName := match[1 : len(match)-1]
		if value, ok := os.LookupEnv(varName); ok {
			return value
		}
		return match
	})
}
