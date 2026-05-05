package cfg

import (
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"reflect"
	"strings"
)

type Get struct {
	Name []string `arg:"" help:"Configuration option to retrieve. Options: ${cfg_opts}." enum:"${cfg_opts}"`
	constant.FlagJSON
}

func (g *Get) Run() error {
	var data map[string]interface{}

	if g.JSON {
		data = make(map[string]interface{}, len(g.Name))
	}

	for _, name := range g.Name {
		if value, err := settings.Get(name); err != nil {
			return fmt.Errorf("error retrieving configuration for %q: %w", name, err)
		} else if g.JSON {
			data[name] = normalizeJSONValue(name, value)
			continue
		} else if len(g.Name) == 1 {
			for _, line := range valueLines(value) {
				fmt.Println(line)
			}
			return nil
		} else {
			lines := valueLines(value)
			for i, line := range lines {
				if i == 0 {
					fmt.Printf("%s: %s\n", name, line)
				} else {
					fmt.Printf("%s: %s\n", strings.Repeat(" ", len(name)), line)
				}
			}
		}
	}

	if g.JSON {
		if out, err := json.MarshalIndent(data, "", "  "); err != nil {
			return fmt.Errorf("error encoding configuration as JSON: %w", err)
		} else {
			fmt.Println(string(out))
		}
	}

	return nil
}

func valueLines(value interface{}) []string {
	if value == nil {
		return []string{"(empty)"}
	}

	if vals, ok := value.([]string); ok {
		if len(vals) == 0 {
			return []string{"(empty)"}
		}
		return vals
	}

	return []string{fmt.Sprint(value)}
}

func normalizeJSONValue(name string, value interface{}) interface{} {
	field, ok := fieldByCfgForGet(name)
	if !ok {
		return value
	}

	if field.Type == reflect.TypeOf([]string{}) {
		if value == nil {
			return []string{}
		}

		switch v := value.(type) {
		case []string:
			out := make([]string, 0, len(v))
			for _, item := range v {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
			if len(out) == 0 {
				return []string{}
			}
			return out
		case string:
			if strings.TrimSpace(v) == "" {
				return []string{}
			}
			parts := strings.Split(v, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					out = append(out, part)
				}
			}
			if len(out) == 0 {
				return []string{}
			}
			return out
		}

		return []string{}
	}

	return value
}

func fieldByCfgForGet(name string) (reflect.StructField, bool) {
	t := reflect.TypeOf(settings.Settings{})
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("cfg") == name {
			return t.Field(i), true
		}
	}

	return reflect.StructField{}, false
}
