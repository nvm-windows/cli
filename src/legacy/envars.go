package legacy

import (
	"common/registry"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	winreg "golang.org/x/sys/windows/registry"
)

const systemEnvKeyPath = `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`

const (
	authorNvmSuffix       = `\author software\nvm`
	authorNvmNodejsSuffix = `\author software\nvm\.nodejs`
)

// RemoveSystemEnvVars removes the NVM v1 SYSTEM-level environment variables
// (NVM_HOME, NVM_SYMLINK) and any references to them in the SYSTEM PATH.
// It also drops community program-root PATH segments (LocalAppData\Author Software\nvm)
// while keeping certified Program Files\Author Software\nvm and the .nodejs shim path.
// Must be called from an elevated process such as MSI custom action or admin remediation.
func RemoveSystemEnvVars() error {
	// Read the variable values before deleting so we can also strip their literal
	// paths from PATH (in case PATH contains expanded paths rather than %VAR% refs).
	nvmHome, _, _ := registry.Get(systemEnvKeyPath + `\NVM_HOME`)
	nvmSymlink, _, _ := registry.Get(systemEnvKeyPath + `\NVM_SYMLINK`)

	registry.Del(
		systemEnvKeyPath+`\NVM_HOME`,
		systemEnvKeyPath+`\NVM_SYMLINK`,
	)

	// Build the set of segments to strip from var refs / expanded values.
	remove := map[string]bool{
		strings.ToLower("%NVM_HOME%"):    true,
		strings.ToLower("%NVM_SYMLINK%"): true,
	}
	if v, ok := nvmHome.(string); ok && v != "" {
		remove[normalizePathSeg(v)] = true
	}
	if v, ok := nvmSymlink.(string); ok && v != "" {
		remove[normalizePathSeg(v)] = true
	}

	// Read and rewrite the SYSTEM PATH. Path is REG_EXPAND_SZ, so common/registry
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

	cleaned := FilterSystemPath(sysPath, remove)
	if cleaned == sysPath {
		return nil
	}

	if err := k.SetExpandStringValue("Path", cleaned); err != nil {
		return err
	}
	broadcastEnvironmentChange()
	return nil
}

// FilterSystemPath removes legacy NVM segments and community program-root entries
// from a PATH string. keepMap entries force-remove specific normalized segments
// (e.g. old NVM_HOME values). Pure function for unit tests.
func FilterSystemPath(sysPath string, forceRemove map[string]bool) string {
	if forceRemove == nil {
		forceRemove = map[string]bool{}
	}
	segments := strings.Split(sysPath, ";")
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		if shouldDropSystemPathSegment(trimmed, forceRemove) {
			continue
		}
		kept = append(kept, seg)
	}
	return strings.Join(kept, ";")
}

func shouldDropSystemPathSegment(seg string, forceRemove map[string]bool) bool {
	norm := normalizePathSeg(seg)
	expanded := normalizePathSeg(os.ExpandEnv(seg))

	// Always keep the per-user .nodejs shim path (certified MSI sets this on Machine PATH),
	// even when legacy NVM_SYMLINK pointed at the same location.
	if isAuthorNvmNodejsPath(norm) || isAuthorNvmNodejsPath(expanded) {
		return false
	}

	// Always keep Program Files certified install root.
	if isProgramFilesAuthorNvmPath(norm) || isProgramFilesAuthorNvmPath(expanded) {
		return false
	}

	if forceRemove[norm] || forceRemove[expanded] {
		return true
	}

	// Drop community / per-user program root: ...\Author Software\nvm (not .nodejs).
	if isAuthorNvmProgramRoot(norm) || isAuthorNvmProgramRoot(expanded) {
		return true
	}

	return false
}

func isAuthorNvmNodejsPath(norm string) bool {
	return strings.HasSuffix(norm, authorNvmNodejsSuffix) ||
		strings.Contains(norm, authorNvmNodejsSuffix+`\`)
}

func isProgramFilesAuthorNvmPath(norm string) bool {
	if !strings.Contains(norm, authorNvmSuffix) {
		return false
	}
	return strings.Contains(norm, `program files`) ||
		strings.Contains(norm, `%programfiles%`) ||
		strings.Contains(norm, `%programfiles(x86)%`)
}

func isAuthorNvmProgramRoot(norm string) bool {
	if isAuthorNvmNodejsPath(norm) {
		return false
	}
	return strings.HasSuffix(norm, authorNvmSuffix)
}

func normalizePathSeg(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	for strings.Contains(normalized, `\\`) {
		normalized = strings.ReplaceAll(normalized, `\\`, `\`)
	}
	normalized = strings.Trim(normalized, `"`)
	return strings.TrimRight(normalized, `\`)
}

func broadcastEnvironmentChange() {
	BroadcastEnvironmentChange()
}

// BroadcastEnvironmentChange notifies running processes that environment variables changed.
func BroadcastEnvironmentChange() {
	var hwndBroadcast uintptr = 0xffff // HWND_BROADCAST
	const wmSettingChange = 0x001A
	env, _ := windows.UTF16PtrFromString("Environment")
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("SendMessageTimeoutW")
	_, _, _ = proc.Call(
		hwndBroadcast,
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(env)),
		0x0002, // SMTO_ABORTIFHUNG
		5000,
		0,
	)
}
