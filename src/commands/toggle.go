package commands

import (
	"common/settings"
	"fmt"
	"nvm/bootstrap"
	"nvm/link"
	"nvm/log"
	"nvm/reshim"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
)

type Toggle struct{}

func (t *Toggle) Run(ctx *kong.Context) error {
	base, err := bootstrap.DataRoot()
	if err != nil {
		log.Error(err)
		return fmt.Errorf("failed to resolve runtime root: %w", err)
	}
	target := filepath.Join(base, ".nodejs")

	switch ctx.Selected().Name {
	case "off":
		if err := link.Unlink(target); err != nil {
			log.Error(err)
		} else {
			if err := settings.Put("enabled", false); err != nil {
				log.Error(err)
				return err
			}

			log.Warn("Node.js version management is now disabled. To enable, run 'nvm on'.")

			fmt.Fprintln(os.Stdout, "NVM for Windows is no longer managing Node.js installations.")
		}

	case "on":
		mode, err := settings.Get("mode")
		if err != nil {
			log.Error(err)
			return err
		}

		target := ".shim"
		if mode == "link" {
			target = ".link/nodejs"

			cfg := settings.Global()

			if cfg.ActiveVersion != "" {
				fmt.Printf("version link: %s --> target: %s\n", filepath.Join(cfg.Root, "v"+cfg.ActiveVersion), filepath.Join(base, target))
				// If an active version is set, ensure the link target is correct
				if err := link.Link(filepath.Join(cfg.Root, "v"+cfg.ActiveVersion), filepath.Join(base, target)); err != nil {
					log.Error(err)
					return err
				}
			}
		}

		if err := link.Link(filepath.Join(base, target), filepath.Join(base, ".nodejs")); err != nil {
			log.Error(err)
			return err
		}

		if mode == "shim" {
			if err := reshim.Run(); err != nil {
				log.Error(err)
				return err
			}
		}

		if err := settings.Put("enabled", true); err != nil {
			log.Error(err)
			return err
		}

		log.Warn("Node.js version management is now enabled. To disable, run 'nvm off'.")

		fmt.Fprintln(os.Stdout, "NVM for Windows is now managing Node.js installations.")
	}

	return nil
}
