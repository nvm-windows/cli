package reshim

import (
	"fmt"
	"nvm/bootstrap"
	"os"
	"os/exec"
)

func Run() error {
	reshimPath, err := bootstrap.UtilityPath("reshim.exe")
	if err != nil {
		return err
	}

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
