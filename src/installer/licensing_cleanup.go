package installer

import (
	"common/preferences"
	"common/registry"
	"strings"
)

// ClearMachineLicensing removes machine-scoped licensing values written by
// `nvm license set` (HKLM preferences). Used during Inno Setup uninstall while
// nvm.exe is still on disk.
func ClearMachineLicensing() error {
	root := strings.TrimRight(strings.TrimSpace(preferences.MACHINE_PREFERENCE_ROOT), "/")
	if root == "" {
		return nil
	}

	for _, valueName := range []string{"AccessToken", "AccessKey", "JwksCose"} {
		if err := registry.Del(root + "/" + valueName); err != nil {
			return err
		}
	}

	return nil
}
