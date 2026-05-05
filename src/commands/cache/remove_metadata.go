package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

type RemoveMetadata struct {
	prompt
}

func (c *RemoveMetadata) Run() error {
	list, err := Store.GetFiles("metadata")
	if err != nil {
		return err
	}

	if len(list) == 0 {
		fmt.Fprintln(os.Stderr, "No files found in cache.")
		return nil
	}

	if c.Prompt {
		source := make([][]string, 0, 0)
		for _, item := range list {
			fmt.Println(item)
			source = append(source, []string{item, filepath.Join(Store.Metadata, item)})
		}

		return promptRemoveSelected(source, "Cached Metadata")
	}

	for _, item := range list {
		if err := os.Remove(filepath.Join(Store.Metadata, item)); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Failed to remove %s: %v\n", item, err)
		} else {
			fmt.Fprintf(os.Stdout, "Removed %s\n", item)
		}
	}

	return nil
}
