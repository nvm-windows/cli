package use

import (
	"common/settings"
	"fmt"
	"os"
)

type Last struct{}

func (c *Last) Run() error {
	lastVersion, err := settings.Get("last_version")
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		return err
	}

	if lastVersion == nil || lastVersion.(string) == "" {
		fmt.Fprintln(os.Stderr, "No previously active version found.")
		return nil
	}

	cmd := Version{}
	cmd.Version = []string{lastVersion.(string)}
	return cmd.Run()
}
