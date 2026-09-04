package commands

import (
	"common/settings"
	"fmt"
	"nvm/constant"
	"nvm/bootstrap"
	"os"
	"os/exec"
	"path/filepath"
)

type Upgrade struct {
	Check bool `flag:"check" help:"Check for updates without performing the upgrade."`
}

func (s *Upgrade) Run() error {
	upgradeDisabled, err := settings.Get("disable_upgrade")
	if err != nil {
		return err
	}

	if upgradeDisabled.(bool) && !s.Check {
		fmt.Println("blocked by computer policy")
		return nil
	}

	sync, err := getSyncToolPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(sync, "upgrade")

	if s.Check {
		cmd.Args = append(cmd.Args, "--check")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(sync)
	cmd.Env = append(os.Environ(), fmt.Sprintf("NVM_EXE_PATH=%s", filepath.Join(filepath.Dir(filepath.Dir(sync)), "nvm.exe")))

	return cmd.Run()
}

type Doctor struct {
	Checks  []string `arg:"" optional:"" help:"Specific checks to run. If not specified, all checks will be run."`
	Autofix bool     `flag:"autofix" help:"Automatically fix issues when possible."`
	List    bool     `flag:"list" help:"List all available checks without running them."`
	constant.FlagJSON
}

func (c *Doctor) Run() error {
	sync, err := getSyncToolPath()
	if err != nil {
		return err
	}

	args := append([]string{"doctor"}, c.Checks...)
	if c.List {
		args = append(args, "--list")
	}
	if c.Autofix {
		args = append(args, "--autofix")
	}
	if c.JSON {
		args = append(args, "--json")
	}

	cmd := exec.Command(sync, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(sync)
	if programRoot, err := bootstrap.ProgramRoot(); err == nil {
		cmd.Env = append(os.Environ(), fmt.Sprintf("NVM_EXE_PATH=%s", filepath.Join(programRoot, "nvm.exe")))
	}

	return cmd.Run()
}

type Reshim struct{}

func (c *Reshim) Run() error {
	sync, err := getSyncToolPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(sync, "reshim")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getSyncToolPath() (string, error) {
	sync, err := bootstrap.UtilityPath("sync.exe")
	if err != nil {
		return "", err
	}

	if _, err := os.Lstat(sync); os.IsNotExist(err) {
		return "", fmt.Errorf("sync.exe not found")
	}

	if _, err := os.Lstat(sync); err != nil {
		if resolved, err := filepath.EvalSymlinks(sync); err == nil {
			sync = resolved
		} else {
			return "", fmt.Errorf("sync.exe could not be resolved (broken symlink?): %w (resolved as %s)", err, sync)
		}
	}

	// Convert to absolute path to avoid relative path issues when working directory changes
	if absSync, err := filepath.Abs(sync); err == nil {
		sync = absSync
	}

	return sync, nil
}

type Subscribe struct {
	Email string `arg:"address" name:"address" help:"The email address to subscribe with." optional:""`
}

func (c *Subscribe) Run() error {
	sync, err := getSyncToolPath()
	if err != nil {
		return err
	}

	args := append([]string{"subscribe"})
	if c.Email != "" {
		args = append(args, c.Email)
	}

	cmd := exec.Command(sync, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = filepath.Dir(sync)

	return cmd.Run()
}
