package utility

import (
	"common/settings"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// format expects setting name first, then value
func DisplaySetting(name, format string) error {
	value, err := settings.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to retrieve default value for %s: %v\n", name, err)
		return err
	}

	var result string
	switch v := value.(type) {
	case string:
		result = v
	case bool:
		result = strconv.FormatBool(v)
	case []string:
		result = strings.Join(v, "\n")
	default:
		result = fmt.Sprintf("%v", v)
	}

	fmt.Fprintf(os.Stdout, format, name, result)
	return nil
}
