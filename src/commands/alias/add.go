package alias

import (
	"common/notify"
	"common/resolver"
	"common/settings"
	"common/system"
	"fmt"
	"strings"
)

var (
	RESERVED_ALIAS_NAMES = map[string]bool{
		"default":    true,
		"current":    true,
		"latest":     true,
		"lts":        true,
		"last":       true,
		"daily":      true, // Reserved for future
		"alpha":      true, // Reserved for future
		"beta":       true, // Reserved for future
		"prerelease": true, // Reserved for future
		"link":       true,
		"shim":       true,
	}
)

type Add struct {
	Name    string `arg:"" help:"Name of the alias." required:""`
	Version string `arg:"" help:"Version to point to." required:""`
	Silent  bool   `flag:"silent" short:"s" help:"Suppress override prompts."`
}

func (c *Add) Run() error {
	if strings.Contains(strings.TrimSpace(c.Name), " ") {
		return fmt.Errorf("alias name cannot contain spaces")
	}

	version, _, err := resolver.Find(c.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve version '%s': %v", c.Version, err)
	}

	alias := strings.ToLower(strings.TrimSpace(c.Name))
	if _, exists := RESERVED_ALIAS_NAMES[alias]; exists || strings.HasPrefix(strings.ReplaceAll(alias, "\\", "/"), "lts/") {
		return fmt.Errorf("cannot use \"%s\" (reserved name)", c.Name)
	}

	source, err := settings.Get("aliases")
	if err != nil {
		return fmt.Errorf("failed to retrieve known aliases: %v", err)
	}

	if source == nil {
		source = []string{}
	}

	aliases := map[string]string{}
	for _, pair := range source.([]string) {
		parts := strings.SplitN(pair, "=", 2)

		if len(parts) == 2 {
			aliases[parts[0]] = parts[1]
		}
	}

	// Check for override if alias already exists
	if alias, exists := aliases[c.Name]; exists {
		if alias == version {
			fmt.Printf("Alias \"%s\" already refers to v%s. No changes made.\n", c.Name, version)
			return nil
		}

		if !c.Silent {
			// Prompt for override
			fmt.Printf("Alias \"%s\" already exists and points to v%s.\nDo you want to override it to point to version \"%s\"? (y/N): ", c.Name, alias, version)
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))

			if response != "y" && response != "yes" {
				fmt.Println("cancelled alias update")
				return nil
			}
		}
	}

	aliases[c.Name] = version

	// Convert back to comma-delimited string
	var raw []string
	for alias, ver := range aliases {
		raw = append(raw, fmt.Sprintf("%s=%s", alias, ver))
	}

	if err := settings.Put("aliases", strings.Join(raw, ",")); err != nil {
		return fmt.Errorf("failed to save alias: %v", err)
	}

	msg := fmt.Sprintf("%q now refers to v%s", c.Name, version)
	fmt.Println(msg)
	if !system.IsAppInForeground() {
		go notify.Send(settings.AppId, "", msg)
	}

	return nil
}
