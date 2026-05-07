package alias

import (
	"common/notify"
	"common/settings"
	"common/system"
	"fmt"
	"strings"
)

type Remove struct {
	Alias []string `arg:"" help:"Name(s) of the alias(es) to remove." required:""`
}

func (c *Remove) Run() error {
	aliases, err := settings.Get("aliases")
	if err != nil {
		return fmt.Errorf("failed to retrieve known aliases: %v", err)
	}

	aliasMap := make(map[string]string)
	for _, pair := range aliases.([]string) {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			aliasMap[parts[0]] = parts[1]
		}
	}

	count := 0
	for _, alias := range c.Alias {
		if _, exists := aliasMap[alias]; !exists {
			fmt.Printf("Alias \"%s\" does not exist.\n", alias)
			continue
		}

		delete(aliasMap, alias)
		count++
	}

	if count == 0 {
		fmt.Println("No aliases removed.")
		return nil
	}

	// Convert back to slice format for storage
	raw := make([]string, 0, len(aliasMap))
	for key, value := range aliasMap {
		raw = append(raw, fmt.Sprintf("%s=%s", key, value))
	}

	if err := settings.Put("aliases", strings.Join(raw, ",")); err != nil {
		return fmt.Errorf("failed to save updated aliases: %v", err)
	}

	suffix := ""
	if count != 1 {
		suffix = "es"
	}
	msg := fmt.Sprintf("%d alias%s removed successfully.", count, suffix)
	fmt.Println(msg)
	if !system.IsAppInForeground() {
		go notify.Send(settings.AppId, "", msg)
	}

	return nil
}
