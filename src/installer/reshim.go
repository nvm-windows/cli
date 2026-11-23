package installer

import (
	"common/notify"
	"common/settings"
	"fmt"
	"os"

	"nvm/bootstrap"
)

func reshim() error {
	return bootstrap.RunReshim("--silent")
}

// printShimLinkGuidance mirrors reshim stderr when hardlink and symlink both fail.
func printShimLinkGuidance() {
	fmt.Fprint(os.Stderr, `
Shim links could not be created (hard link and symlink both failed).
Hard links require .shim and proxy.exe on the same volume; symlinks need
Developer Mode or the Create symbolic links privilege.

Try one of:
  1) Keep Node storage on the same drive as NVM, for example:
       nvm config set root=%LOCALAPPDATA%\Author Software\nvm\installs
  2) Switch to link mode:
       nvm config set mode=link
       nvm use link
  3) Enable Developer Mode (symlink privilege):
       https://docs.nvm-windows.com/features/modes
`)
}

// finalizeInstallShims runs reshim after version installs, discarding backups only
// on success. On failure it rolls back newly installed dirs and hard-fails.
func finalizeInstallShims(txns []*Transaction, status *Status, modulesInstalled bool, hadInstalledVersions bool) error {
	if status.TotalInstalled > 0 || modulesInstalled {
		// Reshim before discarding install backups / activating, so a broken
		// shim set can roll back newly installed version dirs.
		if err := runReshim(); err != nil {
			_ = notify.HelpEnablingSymlinks(settings.AppId)
			printShimLinkGuidance()
			status.Cancel("Installation rolled back: shim links could not be created.")
			for _, txn := range txns {
				rollbackCanceledInstall(txn, status)
			}
			return status.Abort(fmt.Errorf("reshim failed: %w", err))
		}

		discardInstallBackups(txns)

		if status.TotalInstalled > 0 {
			if err := activateDefaultIfFirstInstall(hadInstalledVersions, status.Versions); err != nil {
				status.Alert(fmt.Errorf("activation warning: %v", err))
			}
		}
		return nil
	}

	discardInstallBackups(txns)
	return nil
}

func discardInstallBackups(txns []*Transaction) {
	for _, txn := range txns {
		if txn != nil && txn.installBackup != "" && !txn.backupDiscarded {
			os.RemoveAll(txn.installBackup)
			txn.backupDiscarded = true
		}
	}
}
