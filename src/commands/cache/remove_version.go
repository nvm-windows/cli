package cache

import (
	"common/resolver"
	"common/settings"
	"fmt"
	"nvm/log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type RemoveVersion struct {
	prompt
	All      bool     `flag:"all" short:"a" help:"Treat version numbers as a range and remove all versions within the range. For example, 20.1 removes 20.1.0, 20.1.2, 20.1.3, etc."`
	Versions []string `arg:"version" optional:"" help:"The version of Node.js to remove from cache (e.g. latest, lts, lts/iron, x.x.x, x.x, x)."`
}

func (c *RemoveVersion) Run() error {
	ok, _ := settings.Get("allow_download_cache_removal")
	switch v := ok.(type) {
	case bool:
		if !v {
			fmt.Fprintln(os.Stderr, "Blocked by this computer's policy.")
			return nil
		}
	}

	if len(c.Versions) == 0 && !c.Prompt {
		fmt.Fprintln(os.Stderr, "No versions specified.")
		return nil
	}

	if c.Prompt {
		list, err := c.resolveVersionFiles()
		if err != nil {
			return err
		}
		return promptRemoveSelected(list, "Cached Versions")
	}

	// Non-prompt: --all expands each version token to all matching cached files.
	if c.All {
		list, err := c.resolveVersionFiles()
		if err != nil {
			return err
		}
		for _, item := range list {
			if err := os.Remove(item[1]); err != nil && !os.IsNotExist(err) {
				err = fmt.Errorf("Failed to remove %s: %v", item[0], err)
				log.Error(err)
				return err
			} else {
				log.Logf("Removed %s from cache", item[0])
				fmt.Fprintf(os.Stdout, "Removed %s\n", item[0])
			}
		}
		return nil
	}

	// Non-prompt, exact resolution via resolver.
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	for _, version := range c.Versions {
		ver, _, err := resolver.Find(version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Version '%s' not found: %v\n", version, err)
			continue
		}

		path := filepath.Join(Store.Versions, fmt.Sprintf("node-v%s-win-%s.7z", ver, arch))
		if err := removeCachedArchiveFileErr(path); err != nil {
			err := fmt.Errorf("Failed to remove version '%s' from cache: %v", ver, err)
			log.Error(err)
			fmt.Fprintf(os.Stderr, "%v\n", err)
		} else {
			log.Logf("Removed v%s from cache", ver)
			fmt.Fprintf(os.Stdout, "Removed v%s\n", ver)
		}
	}

	return nil
}

// resolveVersionFiles builds the [label, path] list used by both the prompt
// and --all paths. When c.All is true and version tokens are provided, only
// cached files whose version matches one of the tokens as a major or
// major.minor prefix are included. When no version tokens are given (or All
// is false without tokens), all cached version files are returned.
func (c *RemoveVersion) resolveVersionFiles() ([][]string, error) {
	files, err := Store.GetFiles("versions")
	if err != nil {
		return nil, err
	}

	// Normalise filter prefixes: strip leading 'v', e.g. "20" or "20.18".
	prefixes := make([]string, 0, len(c.Versions))
	for _, v := range c.Versions {
		prefixes = append(prefixes, strings.TrimPrefix(strings.ToLower(v), "v"))
	}

	list := make([][]string, 0, 0)
	for _, file := range files {
		ver, err := extractVersionFromFilename(file)
		if err != nil {
			continue
		}

		if c.All && len(prefixes) > 0 && !matchesAnyPrefix(ver, prefixes) {
			continue
		}

		list = append(list, []string{"v" + ver, filepath.Join(Store.Versions, file)})
	}

	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}

	return list, nil
}

// matchesAnyPrefix returns true when ver (e.g. "20.18.1") starts with any of
// the supplied prefixes (e.g. "20" or "20.18") at a version-component boundary.
func matchesAnyPrefix(ver string, prefixes []string) bool {
	lower := strings.ToLower(ver)
	for _, p := range prefixes {
		if lower == p {
			return true
		}
		// Ensure boundary: "20" must match "20.x.x" not "200.x.x".
		if strings.HasPrefix(lower, p+".") {
			return true
		}
	}
	return false
}
