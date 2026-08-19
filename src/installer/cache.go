package installer

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func versionDownloadCachePath(version, cacheDir string) string {
	if cacheDir == "" {
		return ""
	}

	cpuarch := runtime.GOARCH
	if cpuarch == "amd64" {
		cpuarch = "x64"
	}
	archiveName := fmt.Sprintf("node-v%s-win-%s.7z", version, cpuarch)
	return filepath.Join(cacheDir, archiveName)
}

func cacheArchivePath(version string, cfg InstallConfig) string {
	if cfg.NoCache || cfg.CacheDir == "" {
		return ""
	}
	return versionDownloadCachePath(version, cfg.CacheDir)
}
