package installer

import (
	"common/notify"
	"common/settings"
	"fmt"
	"os"
	"strings"

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
  1) Enable Developer Mode (symlink privilege), then:
       nvm reshim --force
       https://docs.nvm-windows.com/features/modes
  2) Switch to link mode:
       nvm config set mode=link
       nvm use link
`)
}

// finalizeInstallShims runs reshim after version installs in shim mode,
// discarding backups only on success. On failure it rolls back newly installed
// dirs and hard-fails. Link mode skips reshim here — ActivateVersion points
// .nodejs at .link/nodejs and does not require package-manager proxy shims.
func finalizeInstallShims(txns []*Transaction, status *Status, modulesInstalled bool, hadInstalledVersions bool) error {
	if status.TotalInstalled > 0 || modulesInstalled {
		if operatingInShimMode() {
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

func operatingInShimMode() bool {
	mode := strings.TrimSpace(strings.ToLower(settings.Global().Mode))
	return mode == "" || mode == "shim"
}

func discardInstallBackups(txns []*Transaction) {
	for _, txn := range txns {
		if txn != nil && txn.installBackup != "" && !txn.backupDiscarded {
			os.RemoveAll(txn.installBackup)
			txn.backupDiscarded = true
		}
	}
}
