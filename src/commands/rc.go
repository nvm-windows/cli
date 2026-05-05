package commands

import (
	"bytes"
	"common/resolver"
	"common/settings"
	"encoding/json"
	"fmt"
	"nvm/installer"
	"nvm/log"
	"nvm/prompt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iancoleman/orderedmap"
)

type RunCommand struct {
	Version     string `arg:"" name:"version" help:"The version to set in the .nvmrc/package.json/etc file." optional:""`
	File        string `flag:"file" short:"f" help:"Determines which file to write the version to." placeholder:".nvmrc"`
	AutoInstall bool   `flag:"auto-install" short:"i" help:"Automatically install the version if it's not already installed."`
	NoInstall   bool   `flag:"no-install" short:"n" help:"Do not automatically install the version if it's not already installed."`
}

func (c *RunCommand) Run() error {
	cfg := settings.Global()

	if c.Version == "" && cfg.ActiveVersion != "" {
		c.Version = cfg.ActiveVersion
	}

	c.Version = strings.Replace(c.Version, "v", "", 1)

	if c.Version == "" {
		return fmt.Errorf("no version specified and no default version specified")
	}

	if c.File == "" {
		c.File = cfg.DefaultDetectFile

		if c.File == "" {
			c.File = ".nvmrc"
		}
	}

	recognized := false
	for _, file := range cfg.AutoDetect {
		if strings.ToLower(file) == strings.ToLower(c.File) {
			recognized = true
			break
		}
	}

	// Prevent unrecognized files from being written, since that can cause confusion and doesn't work.
	if !recognized {
		return fmt.Errorf("%s would not be recognized. Must be one of: %s", c.File, strings.Join(cfg.AutoDetect, ", "))
	}

	// normalize version
	node_version, npm_version, err := resolver.Find(c.Version)
	if err != nil {
		return err
	}

	installed, _, err := resolver.IsInstalled(node_version)
	if err != nil {
		return err
	}

	autoInstall := cfg.AutoInstall
	if c.AutoInstall {
		autoInstall = true
	}
	if c.NoInstall {
		autoInstall = false
	}

	if !installed {
		if autoInstall {
			cont := true

			if cfg.AutoInstallPrompt {
				ok, err := prompt.Confirm(fmt.Sprintf("Node.js v%s is not installed. Do you want to install it now?", node_version), "y")
				if err != nil {
					return fmt.Errorf("failed to read auto-install prompt: %w", err)
				}

				if !ok {
					if answer, err := prompt.Confirm(fmt.Sprintf("Do you want to continue setting the run command to Node.js v%s even though it is not installed?", node_version), "n"); err != nil {
						return fmt.Errorf("failed to read invalid RC prompt: %w", err)
					} else if !answer {
						return fmt.Errorf("canceled for v%s", node_version)
					} else {
						cont = false
					}
				}
			}

			if cont {
				fmt.Printf("Installing Node.js v%s...\n", node_version)

				if err := installer.Install(installer.InstallConfig{
					Versions: []string{node_version},
				}); err != nil {
					log.Error(err)
					return fmt.Errorf("failed to install v%s: %w", node_version, err)
				}
			}
		} else {
			defer func() {
				fmt.Printf("Node.js v%s is not installed.\nUse 'nvm install %s' to install it.\n", node_version, node_version)
			}()
		}
	} else {
		content, err := os.ReadFile(filepath.Join(cfg.Root, "v"+node_version, "node_modules", "npm", "package.json"))
		if err != nil {
			return fmt.Errorf("failed to read npm package.json for v%s: %w", node_version, err)
		}

		var npmInfo struct {
			Version string `json:"version"`
		}

		if err := json.Unmarshal(content, &npmInfo); err != nil {
			return fmt.Errorf("failed to parse npm package.json for v%s: %w", node_version, err)
		}

		npm_version = npmInfo.Version
	}

	switch strings.ToLower(c.File) {
	case "package.json":
		if _, err := os.ReadFile(c.File); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("package.json not found in the current directory")
			}

			return err
		}

		content, err := os.ReadFile(c.File)
		if err != nil {
			return fmt.Errorf("failed to read package.json: %w", err)
		}

		var data orderedmap.OrderedMap
		if err := json.Unmarshal(content, &data); err != nil {
			return fmt.Errorf("failed to parse package.json: %w", err)
		}

		engines := orderedmap.New()
		engines.Set("node", node_version)
		engines.Set("npm", npm_version)
		data.Set("engines", engines)

		var out bytes.Buffer
		enc := json.NewEncoder(&out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("failed to serialize package.json: %w", err)
		}

		newContent := bytes.TrimSuffix(out.Bytes(), []byte("\n"))
		if err := os.WriteFile(c.File, newContent, 0644); err != nil {
			return fmt.Errorf("failed to write package.json: %w", err)
		}

		fmt.Printf("Successfully set %s Node.js engine to Node.js v%s (with npm v%s)\n", c.File, node_version, npm_version)

	default:
		if err := os.WriteFile(c.File, []byte(node_version), 0644); err != nil {
			return fmt.Errorf("failed to write version to %s: %w", c.File, err)
		}

		fmt.Printf("Successfully set %s Node.js version to v%s\n", c.File, node_version)
	}

	return nil
}
