package cache

import (
	"fmt"
	"os"
)

type Clear struct{}

func (c *Clear) Run() error {
	if err := os.RemoveAll(Store.Metadata); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "failed to clear metadata cache: %v\n", err)
	} else {
		fmt.Fprintln(os.Stdout, "cleared metadata cache.")
	}

	if err := os.RemoveAll(Store.Versions); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "failed to clear versions cache: %v\n", err)
	} else {
		fmt.Fprintln(os.Stdout, "cleared versions cache.")
	}

	return nil
}
