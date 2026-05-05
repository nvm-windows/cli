package list

import (
	"common/resolver"
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

type Installs struct {
	constant.FlagLimits
	constant.FlagJSON
	constant.ArgMajors
}

func (c *Installs) Run() error {
	// Local installed versions
	cfg := settings.Global()
	installRoot := settings.Expand(cfg.Root)

	entries, err := os.ReadDir(installRoot)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No versions installed.")
			return nil
		}
		return err
	}

	var installed []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "v") {
			continue
		}
		vnum := strings.TrimPrefix(name, "v")

		nodeExe := filepath.Join(installRoot, name, "node.exe")
		if _, err := os.Stat(nodeExe); err != nil {
			continue
		}

		// Apply major filter if specified.
		if len(c.Majors) > 0 {
			major := strings.SplitN(vnum, ".", 2)[0]
			matched := false
			for _, m := range c.Majors {
				mParts := strings.SplitN(strings.TrimPrefix(m, "v"), ".", 2)
				if major == mParts[0] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		installed = append(installed, vnum)
	}

	// Sort descending by semver.
	sort.Slice(installed, func(i, j int) bool {
		return semverGreater(installed[i], installed[j])
	})

	if len(installed) == 0 {
		if len(c.Majors) > 0 {
			_, err := strconv.Atoi(strings.Split(strings.TrimPrefix(strings.ToLower(c.Majors[0]), "v"), ".")[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Filters cannot use named aliases (%s). Use a major version number to filter the list.", c.Majors[0])
				return nil
			}
		}
		fmt.Println("No versions installed.")
		return nil
	}

	active := cfg.ActiveVersion

	if c.JSON {
		cached := getCachedVersionsMap()
		activeVersion := strings.ToLower(strings.TrimPrefix(active, "v"))

		metadata := map[string][]string{}
		if versions, err := resolver.List(c.Majors...); err == nil {
			for _, v := range versions {
				if len(v) >= 5 {
					metadata[strings.ToLower(v[0])] = v
				}
			}
		}

		out := make([]nodeVersion, len(installed))
		for i, v := range installed {
			versionLower := strings.ToLower(v)
			versionKey := "v" + versionLower

			entry := nodeVersion{
				Version:   v,
				Date:      nil,
				NPM:       nil,
				LTS:       false,
				Codename:  nil,
				Security:  nil,
				Installed: true,
				Cached:    cached[versionKey],
				System:    versionLower == activeVersion,
			}

			if m, ok := metadata[versionLower]; ok {
				ltsRaw := m[3]
				ltsLower := strings.ToLower(ltsRaw)
				codename := interface{}(nil)
				if ltsRaw != "" && ltsLower != "true" && ltsLower != "false" {
					codename = ltsRaw
				}

				entry.Date = nullableString(m[1])
				entry.NPM = nullableString(m[2])
				entry.LTS = ltsRaw != ""
				entry.Codename = codename
				entry.Security = nullableBoolOrString(m[4])
			}

			out[i] = entry
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

	for _, v := range installed {
		if v == active {
			fmt.Fprintf(w, "* %s\t(default)\n", v)
		} else {
			fmt.Fprintf(w, "  %s\t\n", v)
		}
	}

	return nil
}
