package installer

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func cacheArchivePath(version string, cfg InstallConfig) string {
	if cfg.NoCache || cfg.CacheDir == "" {
		return ""
	}

	cpuarch := runtime.GOARCH
	if cpuarch == "amd64" {
		cpuarch = "x64"
	}
	archiveName := fmt.Sprintf("node-v%s-win-%s.7z", version, cpuarch)
	return filepath.Join(cfg.CacheDir, archiveName)
}
