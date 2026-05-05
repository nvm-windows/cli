package list

import (
	"common/resolver"
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"
)

type Releases struct {
	constant.FlagLimits
	constant.FlagJSON
	constant.ArgMajors
}

func (c *Releases) Run(ctx *kong.Context) error {
	warning := ""

	switch ctx.Selected().Name {
	case "available":
		warning = "\nNote: The 'available' argument is deprecated. Please use 'releases' instead."
	case "list-remote":
		warning = fmt.Sprintf("\nNote: '%s' command not found. Ran 'list releases' instead.\n", ctx.Args[0])
	}

	if !c.JSON && warning != "" {
		defer func() {
			fmt.Fprint(os.Stderr, warning)
		}()
	}

	versions, err := resolver.List(c.Majors...)
	if err != nil {
		return err
	}

	// Get installed and cached versions for marking
	installed := getInstalledVersionsMap()
	cached := getCachedVersionsMap()
	activeVersion := strings.ToLower(strings.TrimPrefix(settings.Global().ActiveVersion, "v"))

	if !c.NoLimit && c.Limit > 0 && len(versions) > c.Limit {
		defer func() {
			fmt.Printf("\n%d versions listed.\n\nUse --no-limit to show all versions or --limit=# to define your own limit.\n", c.Limit)
		}()

		versions = versions[:c.Limit]
	}

	if c.JSON {
		entries := make([]nodeVersion, len(versions))
		for i, v := range versions {
			version := strings.ToLower(v[0])
			versionKey := "v" + version
			ltsRaw := v[3]
			ltsLower := strings.ToLower(ltsRaw)
			codename := interface{}(nil)
			if ltsRaw != "" && ltsLower != "true" && ltsLower != "false" {
				codename = ltsRaw
			}

			entries[i] = nodeVersion{
				Version:   nullableString(v[0]),
				Date:      nullableString(v[1]),
				NPM:       nullableString(v[2]),
				LTS:       ltsRaw != "",
				Codename:  codename,
				Security:  nullableBoolOrString(v[4]),
				Installed: installed[versionKey],
				Cached:    cached[versionKey],
				System:    version == activeVersion,
			}
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(entries); err != nil {
			return err
		}
		_ = os.Stdout.Sync()

		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		w.Flush()
	}()

	fmt.Fprintf(w, "\tVersion\tReleased\t\n")
	fmt.Fprintf(w, "\t-------\t----------\t\n")

	// previousMajor := ""
	for _, v := range versions {
		version := strings.ToLower(v[0])
		versionKey := "v" + version
		// major := strings.SplitN(v[0], ".", 2)[0]
		// if previousMajor != "" && major != previousMajor {
		// 	fmt.Fprintf(w, "\n")
		// }

		// ltsStatus := ""
		// if v[3] != "" {
		// 	ltsStatus = "yes"
		// 	if lower := strings.ToLower(v[3]); lower != "true" && lower != "false" {
		// 		ltsStatus = v[3]
		// 	}
		// }

		status := []string{}
		if installed[versionKey] {
			status = append(status, "installed")
		}

		if cached[versionKey] {
			status = append(status, "cached")
		}

		switch strings.ToLower(v[4]) {
		case "true":
			status = append(status, "security release")
		case "false":
			status = append(status, "no")
		default:
			if len(strings.TrimSpace(v[4])) > 0 {
				status = append(status, v[4])
			}
		}

		active := ""
		if version == activeVersion {
			active = "*"
			status = append(status, "current default")
		}

		fmt.Fprintf(w, "%s\tv%s\t%s\t%s\t\n", active, v[0], v[1], strings.Join(status, ", "))
		// previousMajor = major
	}

	return nil
}
