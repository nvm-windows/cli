package installer

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"common/settings"
)

func healInstalledVersionVisibility(extraPaths ...string) {
	for _, path := range extraPaths {
		clearHiddenDir(path)
	}

	root := expand(settings.Global().Root)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(entry.Name()), "v") {
			continue
		}
		clearHiddenDir(filepath.Join(root, entry.Name()))
	}
}

func clearHiddenDir(path string) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil || attrs&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		return
	}
	_ = syscall.SetFileAttributes(ptr, attrs&^syscall.FILE_ATTRIBUTE_HIDDEN)
}
