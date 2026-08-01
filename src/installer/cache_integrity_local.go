package installer

import (
	prefs "common/preferences"
	"common/registry"
	"common/settings"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var errLocalSHASUMUnavailable = errors.New("local SHASUM file not available")

var verifyArchiveWithLocalSHASUMFn = verifyArchiveWithLocalSHASUM
var isMachinePolicyTrustedLocalArchiveFn = isMachinePolicyTrustedLocalArchive

func localSHASUMPath(version, archivePath string) string {
	cpuarch := runtime.GOARCH
	if cpuarch == "amd64" {
		cpuarch = "x64"
	}

	targetDir := filepath.Dir(archivePath)
	if targetDir == "" {
		targetDir = "."
	}

	return filepath.Join(targetDir, fmt.Sprintf("SHASUMS256-v%s-win-%s.txt", version, cpuarch))
}

func verifyArchiveWithLocalSHASUM(version, archivePath string) error {
	shasumPath := localSHASUMPath(version, archivePath)
	if _, err := os.Stat(shasumPath); err != nil {
		if os.IsNotExist(err) {
			return errLocalSHASUMUnavailable
		}
		return err
	}

	verified, err := verifyNodeSHASUM(archivePath, shasumPath)
	if err != nil {
		return err
	}
	if !verified {
		return fmt.Errorf("SHASUM verification failed")
	}

	return nil
}

func isMachinePolicyTrustedLocalArchive(archivePath string) (bool, error) {
	policyRoot := strings.TrimRight(strings.TrimSpace(prefs.MACHINE_POLICY_ROOT), "/")
	if policyRoot == "" {
		return false, nil
	}

	localOnly, exists, err := registry.GetBool(policyRoot + "/LocalInstallOnly")
	if err != nil {
		return false, err
	}
	if !exists || !localOnly {
		return false, nil
	}

	dirValue, exists, err := registry.Get(policyRoot + "/LocalInstallDir")
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}

	dirStr, ok := dirValue.(string)
	if !ok || strings.TrimSpace(dirStr) == "" {
		return false, nil
	}

	return pathWithinRoot(archivePath, settings.Expand(strings.TrimSpace(dirStr)))
}

func pathWithinRoot(path, root string) (bool, error) {
	path = strings.TrimSpace(path)
	root = strings.TrimSpace(root)
	if path == "" || root == "" {
		return false, nil
	}

	pathAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false, fmt.Errorf("unable to resolve archive path: %w", err)
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false, fmt.Errorf("unable to resolve trusted install path: %w", err)
	}

	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}

	return true, nil
}

func finishDownloadCacheVerification(version, archivePath string) error {
	if err := signDownloadArchiveCacheFn(archivePath); err != nil {
		return fmt.Errorf("unable to sign download cache for v%s: %w", version, err)
	}
	removeLegacyCacheDigestSidecar(archivePath)
	return nil
}
