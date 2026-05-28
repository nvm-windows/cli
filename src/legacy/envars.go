package legacy

import (
	"common/registry"
	"strings"

	winreg "golang.org/x/sys/windows/registry"
)

const systemEnvKeyPath = `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

// RemoveLegacySystemEnvVars removes the NVM v1 SYSTEM-level environment variables
// (NVM_HOME, NVM_SYMLINK) and any references to them in the SYSTEM PATH.
// Must be called from an elevated process such as admin remediation tooling.
func RemoveSystemEnvVars() error {
	// Read the variable values before deleting so we can also strip their literal
	// paths from PATH (in case PATH contains expanded paths rather than %VAR% refs).
	nvmHome, _, _ := registry.Get(systemEnvKeyPath + `\NVM_HOME`)
	nvmSymlink, _, _ := registry.Get(systemEnvKeyPath + `\NVM_SYMLINK`)

	registry.Del(
		systemEnvKeyPath+`\NVM_HOME`,
		systemEnvKeyPath+`\NVM_SYMLINK`,
	)

	// Build the set of segments to strip.
	remove := map[string]bool{
		strings.ToLower("%NVM_HOME%"):    true,
		strings.ToLower("%NVM_SYMLINK%"): true,
	}
	if v, ok := nvmHome.(string); ok && v != "" {
		remove[strings.ToLower(strings.TrimRight(v, `\/`))] = true
	}
	if v, ok := nvmSymlink.(string); ok && v != "" {
		remove[strings.ToLower(strings.TrimRight(v, `\/`))] = true
	}

	// Read and rewrite the SYSTEM PATH.  Path is REG_EXPAND_SZ, so common/registry
	// cannot write it back correctly — use winreg directly for the SET_VALUE call.
	k, err := winreg.OpenKey(winreg.LOCAL_MACHINE,
		`SYSTEM\CurrentControlSet\Control\Session Manager\Environment`,
		winreg.QUERY_VALUE|winreg.SET_VALUE)
	if err != nil {
		return nil // not elevated or key missing — var deletion already done
	}
	defer k.Close()

	sysPath, _, err := k.GetStringValue("Path")
	if err != nil {
		return nil // PATH unreadable; var deletion already succeeded
	}

	segments := strings.Split(sysPath, ";")
	kept := segments[:0]
	for _, seg := range segments {
		norm := strings.ToLower(strings.TrimRight(strings.TrimSpace(seg), `\/`))
		if !remove[norm] {
			kept = append(kept, seg)
		}
	}

	cleaned := strings.Join(kept, ";")
	if cleaned == sysPath {
		return nil
	}

	return k.SetExpandStringValue("Path", cleaned)
}
