package installer

import (
	"fmt"
	"nvm/bootstrap"
	"os"
	"os/exec"
)

func runReshimCommand(path string, args ...string) error {
	args = append(args, "--silent")
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func reshim() error {
	utilPath, err := bootstrap.UtilityPath("sync.exe")
	if err != nil {
		return err
	}

	if _, err := os.Stat(utilPath); err != nil {
		return fmt.Errorf("sync.exe not found at %s: %w", utilPath, err)
	}

	if err := runReshimCommand(utilPath, "reshim"); err != nil {
		return fmt.Errorf("failed to reshim: %w", err)
	}

	return nil
}
