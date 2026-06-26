package commands

import (
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"
)

type Default struct {
	constant.FlagJSON
}

func (c *Default) Run(ctx *kong.Context, vars kong.Vars) error {
	if ctx.Command() == "current" {
		defer func() {
			fmt.Printf("\nThe \"%s current\" command is now \"%s default\"\n", vars["app"], vars["app"])
		}()
	}

	cfg := settings.Global()

	active := cfg.ActiveVersion
	if strings.TrimSpace(active) == "" {
		active = "none"
	}

	if c.JSON {
		out := map[string]string{"default": active}
		if cfg.LastVersion != "" {
			out["last"] = cfg.LastVersion
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}

		fmt.Println(string(data))

		return nil
	}

	t := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	current := "none"
	if active != "" {
		current = "v" + active
	}

	fmt.Fprintf(t, "Default\t: %s\n", current)

	if cfg.LastVersion != "" {
		fmt.Fprintf(t, "Last\t: v%s\n", cfg.LastVersion)
	}

	t.Flush()

	return nil
}
