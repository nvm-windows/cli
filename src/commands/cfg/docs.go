package cfg

import (
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"os"
	"reflect"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"
)

type Docs struct {
	constant.FlagJSON
}

type optionInfo struct {
	Label       string   `json:"label"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Default     string   `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

func (i *Docs) Run() error {
	t := reflect.TypeOf(settings.Settings{})
	options := make([]optionInfo, 0, t.NumField())

	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		name := field.Tag.Get("cfg")
		if name == "" || isHiddenOption(field, name) {
			continue
		}

		defaultValue := field.Tag.Get("default")
		if strings.TrimSpace(defaultValue) == "" {
			defaultValue = "(none)"
		}

		description := field.Tag.Get("help")
		if strings.TrimSpace(description) == "" {
			description = "(none)"
		}

		var enumValues []string
		if raw := field.Tag.Get("enum"); raw != "" {
			for _, v := range strings.Split(raw, ",") {
				if v = strings.TrimSpace(v); v != "" {
					enumValues = append(enumValues, v)
				}
			}
		}

		options = append(options, optionInfo{
			Label:       labelForField(field),
			Name:        name,
			Description: description,
			Default:     defaultValue,
			Enum:        enumValues,
		})
	}

	sort.Slice(options, func(left, right int) bool {
		return options[left].Name < options[right].Name
	})

	if i.JSON {
		out, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}

		fmt.Fprintln(os.Stdout, string(out))

		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	const wrapWidth = 72

	for i, option := range options {
		if i > 0 {
			fmt.Fprint(w, "\t\n")
		}
		fmt.Fprintf(w, "%s\t: %s\n", option.Label, option.Name)

		for _, line := range wrapInfoWords(option.Description, wrapWidth) {
			fmt.Fprintf(w, "\t  %s\n", line)
		}

		if len(option.Enum) > 0 {
			enumLine := "Options: " + strings.Join(option.Enum, ", ")
			for j, line := range wrapInfoWords(enumLine, wrapWidth) {
				if j == 0 {
					fmt.Fprintf(w, "\t  %s\n", line)
				} else {
					fmt.Fprintf(w, "\t               %s\n", line)
				}
			}
		}

		if len(option.Default) > 0 {
			fmt.Fprintf(w, "\t  Defaults to %s\n", option.Default)
		}
	}

	fmt.Println(`The following configuration options can be set using "nvm cfg set <key>=<value>"` + "\n\n")

	return nil
}

func wrapInfoWords(text string, maxLen int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= maxLen {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	return append(lines, current)
}

func isHiddenOption(field reflect.StructField, name string) bool {
	return field.Tag.Get("hidden") == "true" || strings.Contains(name, "active_version")
}

func labelForField(field reflect.StructField) string {
	var builder strings.Builder
	runes := []rune(field.Name)

	for idx, r := range runes {
		if idx > 0 && unicode.IsUpper(r) {
			prev := runes[idx-1]
			nextLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
			if unicode.IsLower(prev) || nextLower {
				builder.WriteRune(' ')
			}
		}
		builder.WriteRune(r)
	}

	label := builder.String()
	label = strings.ReplaceAll(label, "Npm", "npm")
	label = strings.ReplaceAll(label, "Node", "Node")
	return label
}
