package main

import (
	"bufio"
	"bytes"
	"common/settings"
	"common/system"
	"fmt"
	"nvm/commands"
	"nvm/installer"
	"nvm/legacy"
	"nvm/log"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

var (
	name            string
	version         string
	description     string
	buildTime       string
	eventSourceName string
)

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "--help")
	}

	switch os.Args[1] {
	case "--register-eventlog":
		// Invoked by the installer via ShellExec('runas', ...) after files are
		// written. Runs elevated so it can write to HKLM. Exit code communicates
		// success/failure back to the installer.
		//
		// Also cleans up NVM v1 SYSTEM env vars (NVM_HOME, NVM_SYMLINK) and any
		// references to them in the SYSTEM PATH — this requires the same elevation.
		legacy.RemoveSystemEnvVars()

		if err := log.RegisterEventSource(eventSourceName); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				return
			}

			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}

		log.Log("Event source registered successfully.")
		return
	case "--register-installed-versions":
		// Invoked by the installer after migration to ensure all migrated
		// versions are registered in Windows Apps the same way normal installs are.
		settings.Load()
		if err := installer.RegisterInstalledVersions(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--unregister-eventlog":
		// Invoked by the uninstaller
		if err := log.UnregisterEventSource(eventSourceName); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "-v", "--version", "version":
		fmt.Printf("v%s\n", version)
		return
	case "-h", "--help", "help":
		// Handled by kong
	default:
		if strings.HasPrefix(os.Args[1], "-") {
			fmt.Printf("Unknown flag: %s\n", os.Args[1])
			os.Exit(1)
		}
	}

	root := any(&commands.Root)
	if os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		root = &commands.RootWithVersion
	}

	if system.IsProcessStartedByExplorer() {
		fmt.Printf("%s v%s should be run from a shell/terminal.\nPress 'Enter' to exit...\n", name, version)
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	settings.Load()

	cli := kong.Parse(
		root,
		kong.Name(name),
		kong.Description(description+"\nv"+version),
		kong.UsageOnError(),
		kong.Vars{
			"app":      name,
			"version":  version,
			"node":     "Node.js",
			"cfg_opts": strings.Join(settings.List(), ", "),
		},
		kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		},
		kong.Help(func(opts kong.HelpOptions, ctx *kong.Context) error {
			// Kong always emits a generic "Run <command> --help" line; remove it
			// so the footer stays concise while preserving the rest of help output.
			buf := &bytes.Buffer{}
			stdout := ctx.Stdout
			ctx.Stdout = buf

			if err := kong.DefaultHelpPrinter(opts, ctx); err != nil {
				ctx.Stdout = stdout
				return err
			}

			ctx.Stdout = stdout
			defaultOutput := buf.String()
			helpHint := ""

			cmd := strings.ToLower(strings.ReplaceAll(os.Args[1], "-", ""))
			if cmd == "help" {
				helpHint = fmt.Sprintf("\nRun \"%s <command> --help\" for more information on a command.\n", name)
				defaultOutput = strings.ReplaceAll(defaultOutput, helpHint, "")

				fmt.Fprint(ctx.Stdout, defaultOutput)
				fmt.Fprintln(os.Stdout, "\nSync Commands:\n  doctor\t       Detect and fix common issues.\n  upgrade\t       Upgrade nvm.")
			} else {
				fmt.Fprint(ctx.Stdout, defaultOutput)
			}

			fmt.Fprintln(os.Stdout, helpHint+"\nAdditional help available at https://docs.nvm-windows.com")

			return nil
		}),
	)

	if err := cli.Run(); err != nil {
		// Print error without Kong's "nvm: error: " prefix
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
