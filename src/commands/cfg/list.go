package cfg

import (
	"common/http"
	prefs "common/preferences"
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/kong"
)

type List struct {
	constant.FlagJSON
}

var hidden = []string{"active_version", "root"}

func (l *List) Run(vars kong.Vars) error {
	data, err := settings.All(prefs.ROOT)

	if err != nil {
		return err
	}

	keys := make([]string, 0, len(data))
	output := map[string]interface{}{}
	for key := range data {
		keys = append(keys, strings.ToLower(key))
	}
	sort.Strings(keys)

	for _, key := range keys {
		if slices.Contains(hidden, key) {
			continue
		}
		output[key] = typedValue(key, data[key])
	}

	if l.JSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	defer func() {
		fmt.Printf("\nrun \"%s config set <option>=<value>[ <option>=<value>]\" to update options.\n", vars["app"])
	}()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// fmt.Fprintf(w, "Option\t  Value\n")
	// fmt.Fprintf(w, "------\t  ------\n")

	for _, key := range keys {
		if value, ok := output[key]; ok {
			if vals, ok := value.([]string); ok {
				if len(vals) == 0 {
					fmt.Fprintf(w, "%s\t: (empty)\n", key)
					continue
				}

				for i, item := range vals {
					if i == 0 {
						fmt.Fprintf(w, "%s\t: %s\n", key, item)
					} else {
						fmt.Fprintf(w, "\t  %s\n", item)
					}
				}
				continue
			}

			fmt.Fprintf(w, "%s\t: %v\n", key, tableValue(value))
		}
	}

	builtinProxy, builtinProxyDetected := http.Proxy("https://nodejs.org/dist/index.tab")
	if builtinProxyDetected {
		fmt.Fprintln(w)
		if output["proxy"] == nil {
			fmt.Fprintf(w, "Note\t: built-in Windows HTTP proxy detected (%s). It will be used when no user proxy is specified.\n", builtinProxy)
		} else {
			fmt.Fprintf(w, "Note\t: built-in Windows HTTP proxy detected (%s). The configured proxy takes precedence.\n", builtinProxy)
		}
	}

	return nil
}

func typedValue(key string, value interface{}) interface{} {
	field, ok := fieldByCfg(key)
	if !ok {
		return value
	}

	if value == nil {
		if field.Type == reflect.TypeOf([]string{}) {
			return []string{}
		}
		return nil
	}

	switch {
	case field.Type.Kind() == reflect.Bool:
		switch v := value.(type) {
		case bool:
			return v
		case uint32:
			return v == 1
		case int:
			return v == 1
		case string:
			s := strings.TrimSpace(strings.ToLower(v))
			if s == "1" || s == "true" {
				return true
			}
			if s == "0" || s == "false" || s == "" {
				return false
			}
		}
		return value

	case field.Type == reflect.TypeOf([]string{}):
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

	default:
		s, ok := value.(string)
		if !ok {
			return value
		}
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return s
	}
}

func tableValue(value interface{}) interface{} {
	if value == nil {
		return "(empty)"
	}

	if vals, ok := value.([]string); ok {
		return strings.Join(vals, ",")
	}

	return value
}

func fieldByCfg(name string) (reflect.StructField, bool) {
	t := reflect.TypeOf(settings.Settings{})
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("cfg") == name {
			return t.Field(i), true
		}
	}

	return reflect.StructField{}, false
}
