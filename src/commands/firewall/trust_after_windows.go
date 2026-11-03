//go:build windows

package firewall

import (
	"common/settings"
	"common/verifycache"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nvm/bootstrap"
)

func init() {
	afterTrustHook = resignTrustedLaunchersAndReshim
}

func resignTrustedLaunchersAndReshim(entries []string) {
	if err := resignTrustedLaunchers(entries); err != nil {
		fmt.Fprintf(os.Stderr, "NVM Firewall: trusted, but could not re-sign launchers: %v\n", err)
	}
	if err := bootstrap.RunReshim("--silent"); err != nil {
		fmt.Fprintf(os.Stderr, "NVM Firewall: trusted, but reshim failed: %v\nRun nvm reshim after the files unlock.\n", err)
	}
}

func resignTrustedLaunchers(entries []string) error {
	var first error
	for _, entry := range entries {
		name := launcherStem(entry)
		if name == "" {
			continue
		}
		for _, path := range findModuleLaunchers(name) {
			if err := verifycache.SignScript(path); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func findModuleLaunchers(name string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(path string) {
		path = filepath.Clean(path)
		key := strings.ToLower(path)
		if _, ok := seen[key]; ok {
			return
		}
		if _, err := os.Stat(path); err != nil {
			return
		}
		seen[key] = struct{}{}
		out = append(out, path)
	}

	cliRels := verifycache.PackageManagerCliRelPathsFor(name)

	root := settings.Expand(settings.Global().Root)
	if root != "" && root != "." {
		entries, err := os.ReadDir(root)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				n := entry.Name()
				if len(n) < 2 || (n[0] != 'v' && n[0] != 'V') {
					continue
				}
				dir := filepath.Join(root, n)
				add(filepath.Join(dir, name+".cmd"))
				add(filepath.Join(dir, name+".bat"))
				for _, rel := range cliRels {
					add(filepath.Join(dir, filepath.FromSlash(rel)))
				}
			}
		}
	}

	if appdata, err := os.UserConfigDir(); err == nil {
		add(filepath.Join(appdata, "npm", name+".cmd"))
		add(filepath.Join(appdata, "npm", name+".bat"))
	}

	// Standalone pnpm (get.pnpm.io): PNPM_HOME\bin\pnpm.cmd (+ optional .mjs)
	if strings.EqualFold(name, "pnpm") {
		if home := strings.TrimSpace(os.Getenv("PNPM_HOME")); home != "" {
			add(filepath.Join(home, "bin", name+".cmd"))
			add(filepath.Join(home, "bin", name+".bat"))
			add(filepath.Join(home, "bin", name+".mjs"))
			add(filepath.Join(home, "bin", name+".cjs"))
			add(filepath.Join(home, "bin", name+".js"))
			add(filepath.Join(home, name+".cmd"))
			add(filepath.Join(home, name+".bat"))
		}
	}
	return out
}
