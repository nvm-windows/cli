package cache

import (
	"fmt"
	"os"
	"strings"

	"common/verifycache"

	"github.com/ncruces/zenity"
)

const legacyCacheDigestSuffix = ".sha256"

func removeCachedArchiveFile(path string) {
	_ = os.Remove(path)
	_ = verifycache.ClearDownloadArchiveCache(path)
	_ = os.Remove(path + legacyCacheDigestSuffix)
}

func removeCachedArchiveFileErr(path string) error {
	removeCachedArchiveFile(path)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("failed to remove %s", path)
	}
	return nil
}

type prompt struct {
	Prompt bool `flag:"prompt" short:"p" help:"Prompt to select specific cached artifacts."`
}

func promptRemoveSelected(items [][]string, title ...string) error {
	label := "Cache"
	if len(title) > 0 {
		label = title[0]
	}

	opts := make([]string, len(items))
	for i, item := range items {
		opts[i] = item[0]
	}

	selected, err := zenity.ListMultiple(
		"Select artifacts to delete:",
		opts,
		zenity.Title(label),
		zenity.OKLabel("Delete"),
		zenity.CancelLabel("Cancel"),
		zenity.CheckList(),
	)
	if err == zenity.ErrCanceled {
		return nil
	}
	if err != nil {
		return err
	}

	count := 0
	for _, item := range items {
		for _, sel := range selected {
			if item[0] == sel {
				count++

				if err := removeCachedArchiveFileErr(item[1]); err != nil {
					fmt.Fprintf(os.Stderr, "failed to remove %s: %v\n", item[1], err)
				} else {
					fmt.Fprintf(os.Stdout, "Removed %s\n", item[0])
				}

				break
			}
		}

		if count == len(selected) {
			break
		}
	}

	return nil
}

func extractVersionFromFilename(file string) (string, error) {
	if strings.HasPrefix(file, "node-v") && strings.HasSuffix(file, ".7z") && strings.Contains(file, "-win-") {
		name := strings.Split(strings.TrimPrefix(file, "node-v"), "-win-")[0]
		return name, nil
	}

	return "", fmt.Errorf("filename '%s' does not match expected version pattern", file)
}
