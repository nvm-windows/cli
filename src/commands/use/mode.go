package use

import (
	"fmt"
	"nvm/mode"
	"os"

	"github.com/alecthomas/kong"
)

type Mode struct{}

func (c *Mode) Run(ctx *kong.Context) error {
	err := mode.Set(ctx.Selected().Name)
	if err != nil {
		// The mode module logs to event viewer, don't duplicate here!
		return err
	}

	fmt.Fprintf(os.Stdout, "successfully switched to %s mode\n", ctx.Selected().Name)

	return nil
}
