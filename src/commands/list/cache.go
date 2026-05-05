package list

import (
	"common/resolver"
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type Cache struct {
	constant.FlagLimits
	constant.FlagJSON
	constant.ArgMajors
}

func (c *Cache) Run() error {
	cachedMap := getCachedVersionsMap()
	installedMap := getInstalledVersionsMap()

	cached := make([]string, 0, len(cachedMap))
	for key := range cachedMap {
		if !strings.HasPrefix(key, "v") {
			continue
		}

		vnum := strings.TrimPrefix(key, "v")

		if len(c.Majors) > 0 {
			major := strings.SplitN(vnum, ".", 2)[0]
			matched := false
			for _, m := range c.Majors {
				mParts := strings.SplitN(strings.TrimPrefix(strings.ToLower(m), "v"), ".", 2)
				if major == mParts[0] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		cached = append(cached, vnum)
	}

	// Sort descending by semver for consistent output.
	sort.Slice(cached, func(i, j int) bool {
		return semverGreater(cached[i], cached[j])
	})

	if len(cached) == 0 {
		fmt.Println("No cached versions.")
		return nil
	}

	activeVersion := strings.ToLower(strings.TrimPrefix(settings.Global().ActiveVersion, "v"))

	if c.JSON {
		metadata := map[string][]string{}
		if versions, err := resolver.List(c.Majors...); err == nil {
			for _, row := range versions {
				if len(row) < 5 {
					continue
				}
				metadata[strings.ToLower(row[0])] = row
			}
		}

		out := make([]nodeVersion, len(cached))
		for i, v := range cached {
			vk := "v" + strings.ToLower(v)

			meta := metadata[strings.ToLower(v)]
			date := ""
			npm := ""
			ltsRaw := ""
			securityRaw := ""
			if len(meta) >= 5 {
				date = meta[1]
				npm = meta[2]
				ltsRaw = meta[3]
				securityRaw = meta[4]
			}

			ltsLower := strings.ToLower(ltsRaw)
			codename := interface{}(nil)
			if ltsRaw != "" && ltsLower != "true" && ltsLower != "false" {
				codename = ltsRaw
			}

			out[i] = nodeVersion{
				Version:   nullableString(v),
				Date:      nullableString(date),
				NPM:       nullableString(npm),
				LTS:       ltsRaw != "",
				Codename:  codename,
				Security:  nullableBoolOrString(securityRaw),
				Installed: installedMap[vk],
				Cached:    true,
				System:    strings.ToLower(v) == activeVersion,
			}
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(out); err != nil {
			return err
		}
		_ = os.Stdout.Sync()
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	for _, v := range cached {
		vk := "v" + strings.ToLower(v)
		marker := ""
		status := []string{}

		if installedMap[vk] {
			status = append(status, "installed")
		}

		if strings.ToLower(v) == activeVersion {
			marker = "*"
			status = append(status, "default")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", marker, v, strings.Join(status, ", "))
	}

	return nil
}
