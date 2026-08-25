package bootstrap

import (
	"common/fs"
	"common/settings"
	"common/verify"
	"fmt"
	"nvm/link"
	"nvm/log"
	"os"
	"path/filepath"
	"strings"
)

const activationBlockedEventCode = 4302

var verifyActivationNode = func(path string) error {
	_, err := verify.VerifyNodeExecutable(path, verify.EffectiveAllowedSigners(settings.Global().AllowedSigners))
	return err
}

var logActivationBlocked = func(versionDir, nodePath, failureKind, detail string) {
	log.ErrorStructured("node.security.activation_blocked", log.StructuredPayload{
		"action":       "activation_blocked",
		"detail":       detail,
		"failure_kind": failureKind,
		"node_path":    nodePath,
		"source":       "link-mode",
		"version_path": versionDir,
	}, activationBlockedEventCode)
}

// ValidateVersionActivation rejects reparse/cross-user-writable version
// directories and requires a trusted node.exe before link-mode activation.
func ValidateVersionActivation(versionDir string) error {
	if err := fs.CheckVersionDirTrust(versionDir); err != nil {
		failureKind := classifyActivationDirectoryFailure(err)
		nodePath := filepath.Join(versionDir, "node.exe")
		logActivationBlocked(versionDir, nodePath, failureKind, err.Error())
		return fmt.Errorf(
			"NVM blocked link-mode activation because the Node.js directory is unsafe.\n\nPath: %s\nReason: %s\nAction: Reinstall this version into a private install root. If this change was unexpected, contact your administrator and review NVM event logs.\nEvent code: NVM%d",
			versionDir, activationDirectoryReason(failureKind), activationBlockedEventCode,
		)
	}
	nodePath := filepath.Join(versionDir, "node.exe")
	if err := verifyActivationNode(nodePath); err != nil {
		logActivationBlocked(versionDir, nodePath, "executable_trust_failed", err.Error())
		return fmt.Errorf(
			"NVM blocked link-mode activation because Node.js integrity could not be verified.\n\nFile: %s\nReason: %s\nAction: %s\nIf this change was unexpected, contact your administrator and review NVM event logs.\nEvent code: NVM%d",
			nodePath, err, activationRepairAction(versionDir), activationBlockedEventCode,
		)
	}
	return nil
}

func activationRepairAction(versionDir string) string {
	version := strings.TrimPrefix(filepath.Base(filepath.Clean(versionDir)), "v")
	if version == "" || version == "." {
		return "Reinstall this version."
	}
	return fmt.Sprintf("Reinstall this version with `nvm install %s --force`.", version)
}

func classifyActivationDirectoryFailure(err error) string {
	detail := strings.ToLower(err.Error())
	switch {
	case strings.Contains(detail, "reparse"), strings.Contains(detail, "symlink"), strings.Contains(detail, "junction"):
		return "reparse_point"
	case strings.Contains(detail, "writable by other users"):
		return "cross_user_writable"
	default:
		return "unsafe_directory"
	}
}

func activationDirectoryReason(failureKind string) string {
	switch failureKind {
	case "reparse_point":
		return "Directory is a junction, symbolic link, or other reparse point."
	case "cross_user_writable":
		return "Directory is writable by other users."
	default:
		return "Directory trust checks failed."
	}
}

func managementEnabled() (bool, error) {
	value, err := settings.Get("enabled")
	if err != nil {
		return true, err
	}
	if value == nil {
		return true, nil
	}

	enabled, ok := value.(bool)
	if !ok {
		return true, nil
	}

	return enabled, nil
}

func ensureActivationLink(mode, dataRoot, installRoot, shimDir, activeVersion string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "shim"
	}

	nodejsPath := filepath.Join(dataRoot, ".nodejs")

	switch mode {
	case "shim":
		if activationLinkMatches(nodejsPath, shimDir) {
			return nil
		}
		if err := link.Link(shimDir, nodejsPath); err != nil {
			return fmt.Errorf("failed to ensure .nodejs junction: %w", err)
		}
		return nil

	case "link":
		linkNodePath := filepath.Join(dataRoot, ".link", "nodejs")
		if version := strings.TrimSpace(activeVersion); version != "" {
			versionDir := filepath.Join(installRoot, "v"+version)
			if !activationLinkMatches(linkNodePath, versionDir) {
				if err := ValidateVersionActivation(versionDir); err != nil {
					return err
				}
				if err := link.Link(versionDir, linkNodePath); err != nil {
					return fmt.Errorf("failed to ensure link-mode target: %w", err)
				}
			}
		}

		if activationLinkMatches(nodejsPath, linkNodePath) {
			return nil
		}
		if err := link.Link(linkNodePath, nodejsPath); err != nil {
			return fmt.Errorf("failed to ensure .nodejs junction: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported operating mode %q", mode)
	}
}

func activationLinkMatches(linkPath, expectedTarget string) bool {
	linkPath = filepath.Clean(linkPath)
	expectedTarget = filepath.Clean(expectedTarget)

	if _, err := os.Lstat(linkPath); err != nil {
		return false
	}

	targetProbe := filepath.Join(expectedTarget, "node.exe")
	linkProbe := filepath.Join(linkPath, "node.exe")

	targetInfo, err := os.Lstat(targetProbe)
	if err != nil {
		return activationLinkMatchesByResolve(linkPath, expectedTarget)
	}

	linkInfo, err := os.Lstat(linkProbe)
	if err != nil {
		return false
	}

	return os.SameFile(targetInfo, linkInfo)
}

func activationLinkMatchesByResolve(linkPath, expectedTarget string) bool {
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return false
	}

	expectedAbs, err := filepath.Abs(expectedTarget)
	if err != nil {
		return false
	}

	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return false
	}

	return strings.EqualFold(resolvedAbs, expectedAbs)
}
