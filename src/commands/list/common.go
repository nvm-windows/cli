package list

import (
	"common/settings"
	"nvm/commands/cache"
	"os"
	"strconv"
	"strings"
)

func semverGreater(a, b string) bool {
	pa := strings.SplitN(a, ".", 3)
	pb := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		var ai, bi int
		if i < len(pa) {
			ai, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			bi, _ = strconv.Atoi(pb[i])
		}
		if ai != bi {
			return ai > bi
		}
	}
	return false
}

type nodeVersion struct {
	Version   interface{} `json:"version"`
	Date      interface{} `json:"date"`
	NPM       interface{} `json:"npm"`
	LTS       bool        `json:"lts"`
	Codename  interface{} `json:"codename"`
	Security  interface{} `json:"security,omitempty"`
	Installed bool        `json:"installed"`
	Cached    bool        `json:"cached"`
	System    bool        `json:"default"`
}

func nullableString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func nullableBoolOrString(value string) interface{} {
	if value == "" {
		return nil
	}
	lower := strings.ToLower(value)
	if lower == "true" {
		return true
	}
	if lower == "false" {
		return false
	}
	return value
}

// getCachedVersionsMap returns a map of cached version numbers (with "v" prefix).
func getCachedVersionsMap() map[string]bool {
	cached := make(map[string]bool)

	path := cache.Store.Versions
	if path == "" {
		return cached
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return cached
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "node-v") && strings.HasSuffix(strings.ToLower(entry.Name()), ".7z") {
			parts := strings.Split(strings.ToLower(entry.Name()), "-")
			if len(parts) > 2 {
				cached[parts[1]] = true
			}
		}
	}

	return cached
}

// getInstalledVersionsMap returns a map of installed version numbers (with "v" prefix).
func getInstalledVersionsMap() map[string]bool {
	installed := make(map[string]bool)

	cfg := settings.Global()
	root := settings.Expand(cfg.Root)
	if root == "" {
		return installed
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return installed
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if strings.HasPrefix(name, "v") {
				installed[name] = true
			}
		}
	}

	return installed
}
