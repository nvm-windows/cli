package cache

import (
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"sort"
	"text/tabwriter"
)

type List struct {
	Name []string `arg:"name" optional:"" help:"Filter by cache name."`
	constant.FlagJSON
}

type cacheJSON struct {
	Label      string   `json:"label"`
	Root       string   `json:"root"`
	TotalFiles int      `json:"total_files"`
	SizeKB     int64    `json:"size_kb"`
	Files      []string `json:"files"`
}

func (c *List) Run() error {
	caches := Store.List()

	// Build sorted name order so output is deterministic.
	type entry struct{ name, label, path string }
	sorted := make([]entry, 0, len(caches))
	for name, v := range caches {
		match := true
		if c.Name != nil && len(c.Name) > 0 {
			match = false
			for _, filter := range c.Name {
				if filter == name {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		if match {
			sorted = append(sorted, entry{name, v[0], v[1]})
		}
	}
	sort.Slice(sorted, func(a, b int) bool {
		return sorted[a].label < sorted[b].label
	})

	if c.JSON {
		result := make(map[string]*cacheJSON, len(sorted))
		for _, e := range sorted {
			cj := &cacheJSON{
				Label: e.label,
				Root:  e.path,
				Files: []string{},
			}
			dirEntries, err := os.ReadDir(e.path)
			if err == nil {
				for _, de := range dirEntries {
					if de.IsDir() {
						continue
					}
					cj.Files = append(cj.Files, de.Name())
					cj.TotalFiles++
					if info, err := de.Info(); err == nil {
						cj.SizeKB += info.Size()
					}
				}
			}
			cj.SizeKB = cj.SizeKB / 1024
			result[e.name] = cj
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	for _, e := range sorted {
		dirEntries, err := os.ReadDir(e.path)
		if err != nil || len(dirEntries) == 0 {
			fmt.Fprintf(w, "%s\t: (empty)\n", e.label)
			continue
		}

		for j, de := range dirEntries {
			if j == 0 {
				fmt.Fprintf(w, "%s\t: %s\n", e.label, de.Name())
			} else {
				fmt.Fprintf(w, "\t  %s\n", de.Name())
			}
		}
	}

	return nil
}
