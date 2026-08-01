package bootstrap

import (
	"common/fs"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"common/verifycache"
)

func runWithShimMaintenance(fn func() error) error {
	shimDir, err := ShimDir()
	if err != nil {
		return err
	}

	proxyPath, err := DataProxyPath()
	if err != nil {
		return err
	}

	return fs.RunWithRuntimeShimWrite(shimDir, proxyPath, fn)
}

// SyncShimAssets copies canonical node/proxy shims into the user data root.
func SyncShimAssets() error {
	return runWithShimMaintenance(func() error {
		programShimPath, err := ProgramShimPath()
		if err != nil {
			return err
		}

		shimDir, err := ShimDir()
		if err != nil {
			return err
		}

		if err := syncShimExecutable(programShimPath, filepath.Join(shimDir, "node.exe")); err != nil {
			return fmt.Errorf("failed to synchronize node shim: %w", err)
		}

		programProxyPath, err := ProgramProxyPath()
		if err != nil {
			return err
		}

		dataProxyPath, err := DataProxyPath()
		if err != nil {
			return err
		}

		if err := syncSharedExecutable(programProxyPath, dataProxyPath); err != nil {
			return fmt.Errorf("failed to synchronize shared proxy shim: %w", err)
		}

		return nil
	})
}

// RunReshim executes reshim.exe inside the shim write window.
func RunReshim(args ...string) error {
	err := runWithShimMaintenance(func() error {
		reshimPath, err := UtilityPath("reshim.exe")
		if err != nil {
			return err
		}

		if _, err := os.Stat(reshimPath); err != nil {
			return fmt.Errorf("reshim not found at %s: %w", reshimPath, err)
		}

		cmd := exec.Command(reshimPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("reshim failed: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	allInstalled := false
	for _, arg := range args {
		if strings.TrimSpace(arg) == "--force" {
			allInstalled = true
			break
		}
	}
	_ = verifycache.PrewarmVerifyCache(allInstalled)
	return nil
}

// MaintainShimDirectory syncs canonical shims and reconciles module hardlinks.
func MaintainShimDirectory() error {
	return runWithShimMaintenance(func() error {
		if err := syncShimAssetsUnlocked(); err != nil {
			return err
		}
		return runReshimUnlocked("--silent")
	})
}

func syncShimAssetsUnlocked() error {
	programShimPath, err := ProgramShimPath()
	if err != nil {
		return err
	}

	shimDir, err := ShimDir()
	if err != nil {
		return err
	}

	if err := syncShimExecutable(programShimPath, filepath.Join(shimDir, "node.exe")); err != nil {
		return fmt.Errorf("failed to synchronize node shim: %w", err)
	}

	programProxyPath, err := ProgramProxyPath()
	if err != nil {
		return err
	}

	dataProxyPath, err := DataProxyPath()
	if err != nil {
		return err
	}

	if err := syncSharedExecutable(programProxyPath, dataProxyPath); err != nil {
		return fmt.Errorf("failed to synchronize shared proxy shim: %w", err)
	}

	return nil
}

func runReshimUnlocked(args ...string) error {
	reshimPath, err := UtilityPath("reshim.exe")
	if err != nil {
		return err
	}

	if _, err := os.Stat(reshimPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reshim not found at %s: %w", reshimPath, err)
	}

	cmd := exec.Command(reshimPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reshim failed: %w", err)
	}

	return nil
}
