//go:build windows

package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	prefs "common/preferences"
	"common/registry"
)

const (
	CommunityAppxName   = "AuthorSoftware.NVMWindows"
	CertifiedAppxName   = "AuthorSoftware.NVMWindows.Certified"
	identityMsixName    = "NVMWindows.msix"
	retiredValueName    = "CommunityIdentityRetired"
	registeredValueName = "CertifiedIdentityRegistered"
)

var runAppxCommand = func(command string) error {
	ps := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	cmd := exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RegisterSparseIdentity registers the per-user identity .msix against programRoot.
// Failure is non-fatal for callers; toasts fall back to AUMID-only.
func RegisterSparseIdentity(programRoot string) error {
	programRoot = filepath.Clean(strings.TrimSpace(programRoot))
	if programRoot == "" {
		return fmt.Errorf("program root is empty")
	}
	msix := filepath.Join(programRoot, "identity", identityMsixName)
	if _, err := os.Stat(msix); err != nil {
		return err
	}
	cmd := fmt.Sprintf(
		"Add-AppxPackage -Path %s -ExternalLocation %s -ErrorAction Stop",
		psSingleQuote(msix),
		psSingleQuote(programRoot),
	)
	return runAppxCommand(cmd)
}

// ProvisionSparseIdentity registers the identity for all users (elevated MSI).
func ProvisionSparseIdentity(programRoot string) error {
	programRoot = filepath.Clean(strings.TrimSpace(programRoot))
	if programRoot == "" {
		return fmt.Errorf("program root is empty")
	}
	msix := filepath.Join(programRoot, "identity", identityMsixName)
	if _, err := os.Stat(msix); err != nil {
		return err
	}
	cmd := fmt.Sprintf(
		"$msix = %s; $root = %s; Add-AppxPackage -Path $msix -ExternalLocation $root -AllUsers -ErrorAction SilentlyContinue; if (-not $?) { Add-AppxPackage -Path $msix -ExternalLocation $root -ErrorAction SilentlyContinue }; try { Add-AppxProvisionedPackage -Online -PackagePath $msix -SkipLicense -ErrorAction SilentlyContinue | Out-Null } catch {}",
		psSingleQuote(msix),
		psSingleQuote(programRoot),
	)
	return runAppxCommand(cmd)
}

// DeprovisionSparseIdentity drops per-user, all-user, and provisioned packages.
func DeprovisionSparseIdentity(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = CertifiedAppxName
	}
	cmd := fmt.Sprintf(
		"$n = %s; Get-AppxPackage -AllUsers -Name $n -ErrorAction SilentlyContinue | Remove-AppxPackage -AllUsers -ErrorAction SilentlyContinue; Get-AppxPackage -Name $n -ErrorAction SilentlyContinue | Remove-AppxPackage -ErrorAction SilentlyContinue; Get-AppxProvisionedPackage -Online -ErrorAction SilentlyContinue | Where-Object { $_.DisplayName -eq $n } | ForEach-Object { Remove-AppxProvisionedPackage -Online -PackageName $_.PackageName -ErrorAction SilentlyContinue }",
		psSingleQuote(name),
	)
	return runAppxCommand(cmd)
}

// UnregisterSparseIdentity removes a sparse identity package. Missing package is success.
func UnregisterSparseIdentity(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = CommunityAppxName
	}
	cmd := fmt.Sprintf(
		"Get-AppxPackage -Name %s | Remove-AppxPackage -ErrorAction SilentlyContinue",
		psSingleQuote(name),
	)
	return runAppxCommand(cmd)
}

// MaybeRemoveStaleCommunityIdentity drops a leftover community Appx after
// community→certified when the MSI impersonated CA ran as SYSTEM.
func MaybeRemoveStaleCommunityIdentity() {
	root, err := ProgramRoot()
	if err != nil || !isMachineProgramRoot(root) {
		return
	}
	if communityIdentityRetired() {
		return
	}
	_ = UnregisterSparseIdentity(CommunityAppxName)
	_ = markCommunityIdentityRetired()
}

// MaybeRegisterCertifiedIdentity registers the Program Files identity .msix once
// per user when the MSI impersonated CA ran as SYSTEM.
func MaybeRegisterCertifiedIdentity() {
	root, err := ProgramRoot()
	if err != nil || !isMachineProgramRoot(root) {
		return
	}
	if certifiedIdentityRegistered() {
		return
	}
	if err := RegisterSparseIdentity(root); err != nil {
		return
	}
	_ = markCertifiedIdentityRegistered()
}

func isMachineProgramRoot(root string) bool {
	low := strings.ToLower(filepath.Clean(root))
	return strings.Contains(low, `\program files\`) || strings.Contains(low, `\program files (x86)\`)
}

func communityIdentityRetired() bool {
	root := strings.TrimSpace(prefs.ROOT)
	if root == "" {
		return false
	}
	v, exists, err := registry.GetBool(root + "/" + retiredValueName)
	if err != nil || !exists {
		return false
	}
	return v
}

func markCommunityIdentityRetired() error {
	root := strings.TrimSpace(prefs.ROOT)
	if root == "" {
		return nil
	}
	return registry.PutBool(true, root+"/"+retiredValueName)
}

func certifiedIdentityRegistered() bool {
	root := strings.TrimSpace(prefs.ROOT)
	if root == "" {
		return false
	}
	v, exists, err := registry.GetBool(root + "/" + registeredValueName)
	if err != nil || !exists {
		return false
	}
	return v
}

func markCertifiedIdentityRegistered() error {
	root := strings.TrimSpace(prefs.ROOT)
	if root == "" {
		return nil
	}
	return registry.PutBool(true, root+"/"+registeredValueName)
}

func psSingleQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}
