package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Confirm(msg string, defaultAnswer ...string) (bool, error) {
	default_value := "y"
	if len(defaultAnswer) > 0 {
		default_value = strings.ToLower(strings.TrimSpace(defaultAnswer[0]))
	}

	reader := bufio.NewReader(os.Stdin)

	if default_value == "y" {
		fmt.Print(msg + " [Y/n] ")
	} else {
		fmt.Print(msg + " [y/N] ")
	}

	text, err := reader.ReadString('\n')
	if err != nil {
		if err == os.ErrClosed {
			return true, nil
		}
		return false, err
	}

	answer := strings.TrimSpace(strings.ToLower(text))
	if answer == "" {
		return (default_value == "y"), nil
	}

	parsed, err := strconv.ParseBool(answer)
	if err == nil {
		return parsed, nil
	}

	if answer == "n" || answer == "no" {
		return false, nil
	}

	if answer == "y" || answer == "yes" {
		return true, nil
	}

	return false, nil
}
