//go:build windows

package installer

import "common/fs"

func ensureCacheDirectoryFS(path string) error {
	return fs.EnsureHiddenDirectory(path)
}
