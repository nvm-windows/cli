package installer

import (
	"common/fs"
	"common/http"
	"common/settings"
	"common/verifycache"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const legacyCacheDigestSuffix = ".sha256"

func invalidateCachedNodeArchive(archivePath string) {
	_ = os.Remove(archivePath)
	_ = verifycache.ClearDownloadArchiveCache(archivePath)
	removeLegacyCacheDigestSidecar(archivePath)
}

func removeLegacyCacheDigestSidecar(archivePath string) {
	_ = os.Remove(archivePath + legacyCacheDigestSuffix)
}

func ensureVersionCacheDirectory(cacheDir string) error {
	if strings.TrimSpace(cacheDir) == "" {
		return nil
	}
	return ensureCacheDirectory(cacheDir)
}

var verifyArchiveWithMirrorSHASUMFn = verifyArchiveWithMirrorSHASUM
var verifyDownloadArchiveCacheFn = verifycache.VerifyDownloadArchiveCache
var signDownloadArchiveCacheFn = verifycache.SignDownloadArchiveCache

func verifyCachedNodeArchiveIntegrity(ctx context.Context, version, archivePath string, cfg InstallConfig) error {
	switch err := verifyDownloadArchiveCacheFn(archivePath); {
	case err == nil:
		removeLegacyCacheDigestSidecar(archivePath)
		return nil
	case errors.Is(err, verifycache.ErrDownloadCacheMiss):
		// fall through to offline-capable verification paths
	default:
		return fmt.Errorf("cached Node.js archive for v%s failed integrity check: %w", version, err)
	}

	switch err := verifyArchiveWithLocalSHASUMFn(version, archivePath); {
	case err == nil:
		return finishDownloadCacheVerification(version, archivePath)
	case errors.Is(err, errLocalSHASUMUnavailable):
		// fall through
	default:
		return fmt.Errorf("cached Node.js archive for v%s failed local SHASUM verification: %w", version, err)
	}

	trusted, err := isMachinePolicyTrustedLocalArchiveFn(archivePath)
	if err != nil {
		return fmt.Errorf("cached Node.js archive for v%s failed trusted local install check: %w", version, err)
	}
	if trusted {
		return finishDownloadCacheVerification(version, archivePath)
	}

	if err := verifyArchiveWithMirrorSHASUMFn(ctx, version, archivePath, cfg); err != nil {
		return err
	}

	return finishDownloadCacheVerification(version, archivePath)
}

func verifyArchiveWithMirrorSHASUM(ctx context.Context, version, archivePath string, cfg InstallConfig) error {
	cpuarch := runtime.GOARCH
	if cpuarch == "amd64" {
		cpuarch = "x64"
	}

	targetDir := filepath.Dir(archivePath)
	if targetDir == "" {
		targetDir = "."
	}

	shasumPath := filepath.Join(targetDir, fmt.Sprintf("SHASUMS256-v%s-win-%s.txt", version, cpuarch))
	_ = os.Remove(shasumPath)
	defer os.Remove(shasumPath)

	insecure := allowInsecureDownloads(cfg)
	var lastErr error

	for _, mirror := range settings.Global().NodeMirror {
		if ctx.Err() != nil {
			return context.Canceled
		}

		shasumURI := fmt.Sprintf("%s/v%s/SHASUMS256.txt", mirror, version)
		shasumJob, err := http.Download(shasumURI, http.DownloadConfig{Cache: true, Destination: shasumPath, AllowInsecure: insecure})
		if err != nil {
			lastErr = err
			continue
		}

		select {
		case <-ctx.Done():
			shasumJob.Cancel()
			return context.Canceled
		case result, ok := <-shasumJob.Result:
			if !ok || result.Error != nil || result.Response == nil || !result.Response.Success {
				if result.Error != nil {
					lastErr = result.Error
				} else {
					lastErr = fmt.Errorf("SHASUM download failed")
				}
				_ = os.Remove(shasumPath)
				continue
			}
		}

		verified, err := verifyNodeSHASUM(archivePath, shasumPath)
		if err != nil {
			lastErr = err
			continue
		}
		if !verified {
			lastErr = fmt.Errorf("SHASUM verification failed")
			continue
		}

		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("SHASUMS256.txt not found on configured mirrors")
	}

	return fmt.Errorf("unable to verify cached Node.js archive for v%s: %w", version, lastErr)
}

func ensureCacheDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return ensureCacheDirectoryFS(path)
}

func copyVerifiedArchiveToCache(sourcePath, cacheFile string) error {
	if err := ensureVersionCacheDirectory(filepath.Dir(cacheFile)); err != nil {
		return err
	}
	if err := fs.CopyFile(sourcePath, cacheFile); err != nil {
		return err
	}
	return signDownloadArchiveCacheFn(cacheFile)
}
