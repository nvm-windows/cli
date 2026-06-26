package installer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const localAppDataEnvName = "LOCALAPPDATA"
const windowsAppsUninstallRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
const nodeVersionUninstallPrefix = "nvm4w-node-v"

var serviceProfileRoots = []string{
	`\windows\system32\config\systemprofile\appdata\local`,
	`\windows\serviceprofiles\localservice\appdata\local`,
	`\windows\serviceprofiles\networkservice\appdata\local`,
}

func CleanupCurrentUserAppData() error {
	var cleanupErrors []error

	if err := removeAllNodeVersionsWindowsAppsEntries(); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to remove node version Windows Apps entries: %w", err))
	}

	dataRoot, ok := currentUserDataRoot(os.LookupEnv)
	if !ok {
		if len(cleanupErrors) > 0 {
			return errors.Join(cleanupErrors...)
		}

		return nil
	}

	if err := os.RemoveAll(dataRoot); err != nil && !os.IsNotExist(err) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to remove current-user AppData root %q: %w", dataRoot, err))
	}

	if len(cleanupErrors) > 0 {
		return errors.Join(cleanupErrors...)
	}

	return nil
}

func removeAllNodeVersionsWindowsAppsEntries() error {
	uninstallRoot, err := registry.OpenKey(registry.CURRENT_USER, windowsAppsUninstallRoot, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer uninstallRoot.Close()

	subKeys, err := uninstallRoot.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}

	var deleteErrors []error
	for _, keyName := range subKeys {
		if !strings.HasPrefix(strings.ToLower(keyName), nodeVersionUninstallPrefix) {
			continue
		}

		fullPath := windowsAppsUninstallRoot + `\` + keyName
		if err := deleteRegistryTree(registry.CURRENT_USER, fullPath); err != nil {
			deleteErrors = append(deleteErrors, fmt.Errorf("%s: %w", fullPath, err))
		}
	}

	if len(deleteErrors) > 0 {
		return errors.Join(deleteErrors...)
	}

	return nil
}

func deleteRegistryTree(root registry.Key, keyPath string) error {
	key, err := registry.OpenKey(root, keyPath, registry.ENUMERATE_SUB_KEYS)
	if err == nil {
		subKeys, readErr := key.ReadSubKeyNames(-1)
		key.Close()
		if readErr != nil {
			return readErr
		}

		for _, subKey := range subKeys {
			childPath := keyPath + `\` + subKey
			if err := deleteRegistryTree(root, childPath); err != nil {
				return err
			}
		}
	}

	if err := registry.DeleteKey(root, keyPath); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "cannot find the file") {
			return nil
		}
		return err
	}

	return nil
}

func currentUserDataRoot(lookupEnv func(string) (string, bool)) (string, bool) {
	localAppData, ok := lookupEnv(localAppDataEnvName)
	if !ok {
		return "", false
	}

	localAppData = strings.TrimSpace(localAppData)
	if localAppData == "" || isServiceProfileLocalAppData(localAppData) {
		return "", false
	}

	return filepath.Join(localAppData, "Author Software", "nvm"), true
}

func isServiceProfileLocalAppData(localAppData string) bool {
	cleaned := strings.ToLower(filepath.Clean(localAppData))
	for _, root := range serviceProfileRoots {
		if strings.HasSuffix(cleaned, root) {
			return true
		}
	}

	return false
}
