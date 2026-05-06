package alias

import (
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"strings"
	"text/tabwriter"
)

type List struct {
	Aliases []string `arg:"" optional:"" help:"Filter the alias list."`
	constant.FlagJSON
}

func (c *List) Run() error {
	raw, err := settings.Get("aliases")
	if err != nil {
		return fmt.Errorf("failed to retrieve aliases: %v", err)
	}

	var data map[string]string
	var t *tabwriter.Writer

	if c.JSON {
		data = make(map[string]string)
	} else {
		t = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	}

	count := 0
	for _, pair := range raw.([]string) {
		parts := strings.SplitN(pair, "=", 2)

		if len(c.Aliases) > 0 {
			match := false

			for _, alias := range c.Aliases {
				if parts[0] == alias {
					match = true
					break
				}
			}

			if !match {
				continue
			}
		}

		if len(parts) == 2 {
			count++

			if c.JSON {
				data[parts[0]] = parts[1]
			} else {
				fmt.Fprintf(t, "%s\t-> v%s\n", parts[0], parts[1])
			}
		}
	}

	if c.JSON {
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal aliases to JSON: %v", err)
		}

		fmt.Println(string(output))

		return nil
	}

	if t != nil {
		t.Flush()
	}

	if count == 0 {
		fmt.Println("No aliases available.")
	}

	return nil
}
