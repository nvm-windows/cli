package installer

import (
	"common/fs"
	"common/verifycache"
	"fmt"
	"nvm/log"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func restoreInstallBackup(txn *Transaction) error {
	if txn == nil || txn.installBackup == "" || txn.backupRestored {
		return nil
	}

	if err := cleanupInstallDir(txn.installDir); err != nil {
		return fmt.Errorf("failed to clean install dir before restoring backup: %w", err)
	}
	if err := os.Rename(txn.installBackup, txn.installDir); err != nil {
		return err
	}
	txn.backupRestored = true
	return nil
}

func rollbackCanceledInstall(txn *Transaction, status *Status) {
	if txn == nil {
		return
	}

	if txn.cachedNew && !txn.cached && txn.cacheFile != "" {
		invalidateCachedNodeArchive(txn.cacheFile)
		log.Logf("Removed Node.js v%s from cache (rollback)", txn.version)
	}

	if txn.installedNew && !txn.installed {
		unregisterNodeVersion(txn.version)
		_ = verifycache.ClearNodeCache(filepath.Join(txn.installDir, "node.exe"))
		if err := cleanupInstallDir(txn.installDir); err != nil {
			status.Alert(fmt.Sprintf("rollback warning for v%s: failed to remove install dir %s: %v", txn.version, txn.installDir, err))
		}
	}

	if txn.installBackup != "" && !txn.backupDiscarded {
		if err := restoreInstallBackup(txn); err != nil {
			status.Alert(fmt.Sprintf("rollback warning for v%s: %v", txn.version, err))
		}
	}
}

func cleanupInstallDir(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	var lastErr error
	for i := 0; i < 6; i++ {
		fs.EnableInheritance(path)
		clearAttributesRecursive(path)

		if err := os.RemoveAll(path); err == nil || os.IsNotExist(err) {
			return nil
		} else {
			lastErr = err
		}

		time.Sleep(time.Duration(150*(i+1)) * time.Millisecond)
	}

	return lastErr
}

func clearAttributesRecursive(root string) error {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}

	for i := len(paths) - 1; i >= 0; i-- {
		p := paths[i]
		if info, err := os.Lstat(p); err != nil {
			continue
		} else if info.IsDir() {
			fs.ClearHidden(p)
			continue
		}

		clearNormal(p)
	}

	return nil
}

func clearNormal(path string) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err == nil {
		syscall.SetFileAttributes(ptr, syscall.FILE_ATTRIBUTE_NORMAL)
	}
}
