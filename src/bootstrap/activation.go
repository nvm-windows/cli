package bootstrap

import (
	"common/settings"
	"fmt"
	"nvm/link"
	"os"
	"path/filepath"
	"strings"
)

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
