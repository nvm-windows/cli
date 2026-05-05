package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

var emptyInt int

func copyModules(ctx context.Context, source_version, node_version string, status *Status) error {
	modules, err := getNewModuleList(source_version, node_version)
	if err != nil {
		return err
	}

	status.NpmLabel = "Copying modules"
	status.Npm = status.Npm + int(len(modules))
	status.Flush()

	defer func() {
		status.Npm = 0
		status.Flush()
	}()

	errs := []error{}
	source := getRoot(source_version)
	target := getRoot(node_version)

	shims, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, name := range modules {
		sourcePath := filepath.Join(source, "node_modules", name)

		if ctx.Err() != nil {
			return context.Canceled
		}

		targetPath := filepath.Join(target, "node_modules", name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			errs = append(errs, err)
			continue
		}

		if err := copyDir(sourcePath, targetPath); err != nil {
			errs = append(errs, err)
			continue
		}

		for _, shim := range shims {
			if !shim.IsDir() && strings.HasPrefix(shim.Name(), name) {
				if err := copyFile(filepath.Join(source, shim.Name()), filepath.Join(target, shim.Name())); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("encountered errors while copying modules: %v", errs)
	}

	return nil
}

func installModulesFrom(ctx context.Context, source_version, node_version string, status *Status) error {
	modules, err := getNewModuleList(source_version, node_version)
	if err != nil {
		return err
	}

	root := getRoot(node_version)
	npm := filepath.Join(root, "npm.cmd")
	prefix := root

	status.NpmLabel = "Installing modules"
	status.Npm = status.Npm + int(len(modules))
	status.Flush()

	defer func() {
		status.Npm = 0
		status.Flush()
	}()

	if err := npmInstall(ctx, npm, prefix, modules, nil, nil); err != nil {
		return err
	}

	return nil
}

func autoInstallModules(ctx context.Context, modules []string, node_version string, status *Status) error {
	slices.Sort(modules)
	prefix := getRoot(node_version)

	status.NpmLabel = "Installing default modules"
	status.Npm = status.Npm + int(len(modules))
	status.Flush()

	defer func() {
		status.Npm = 0
		status.Flush()
	}()

	if err := npmInstall(ctx, filepath.Join(prefix, "npm.cmd"), prefix, modules, nil, nil); err != nil {
		return err
	}
	return nil
}

func npmInstall(
	ctx context.Context,
	npmPath string,
	prefix string,
	modules []string,
	onModuleStart func(module string, current int, total int),
	onModuleComplete func(module string, done int, total int),
) error {
	npmDir := filepath.Dir(npmPath)
	nodePath := filepath.Join(npmDir, "node.exe")
	npmCliJS := filepath.Join(npmDir, "node_modules", "npm", "bin", "npm-cli.js")

	total := len(modules)
	for i, module := range modules {
		if ctx.Err() != nil {
			return context.Canceled
		}
		if onModuleStart != nil {
			onModuleStart(module, i+1, total)
		}

		cmd := exec.CommandContext(ctx, nodePath, npmCliJS, "install", "-g", "--prefix", prefix, module)
		cmd.Dir = npmDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				return context.Canceled
			}

			msg := strings.TrimSpace(string(output))
			if msg == "" {
				return fmt.Errorf("failed to install module %s: %w", module, err)
			}

			const maxLogBytes = 4096
			if len(msg) > maxLogBytes {
				msg = msg[len(msg)-maxLogBytes:]
			}

			return fmt.Errorf("failed to install %s module: %w\n---- npm output ----\n%s", module, err, msg)
		}

		if onModuleComplete != nil {
			onModuleComplete(module, i+1, total)
		}
	}

	return nil
}

func getModuleList(version string) (map[string]string, error) {
	root := filepath.Join(getRoot(version), "node_modules")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	modules := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".bin" {
			continue
		}
		if strings.HasPrefix(name, "@") {
			scoped, err := os.ReadDir(filepath.Join(root, name))
			if err != nil {
				continue
			}
			for _, s := range scoped {
				if s.IsDir() {
					modName := filepath.Join(name, s.Name())
					modules[modName] = filepath.Join(root, modName)
				}
			}
			continue
		}
		modules[name] = filepath.Join(root, name)
	}

	return modules, nil
}

func getNewModuleList(source_version, node_version string) ([]string, error) {
	source, err := getModuleList(source_version)
	if err != nil {
		return nil, err
	}

	existing, err := getModuleList(node_version)
	if err != nil {
		return nil, err
	}

	modules := []string{}
	for name := range source {
		if _, exists := existing[name]; !exists {
			modules = append(modules, name)
		}
	}

	slices.Sort(modules)

	return modules, nil
}

func copyDir(src, dst string) error {
	// /E: Subdirectories (incl. empty)
	// /MT:32: Multi-threaded (up to 128)
	// /R:0 /W:0: Skip retries for speed
	// /NP: No progress percentage (cleaner logs)
	cmd := exec.Command("robocopy", src, dst, "/E", "/MT:32", "/R:0", "/W:0", "/NP")

	err := cmd.Run()

	// Parse custom Robocopy exit codes
	if err, ok := err.(*exec.ExitError); ok {
		ws := err.Sys().(syscall.WaitStatus)
		exitCode := ws.ExitStatus()

		if exitCode < 8 {
			return nil
		} else {
			if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
				return fmt.Errorf("copy failed with exit code: %d", exitCode)
			}
		}
	} else if err != nil {
		return fmt.Errorf("system error: %v", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
