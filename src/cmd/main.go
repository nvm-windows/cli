package main

import (
	"bufio"
	"bytes"
	"common/fs"
	"common/license"
	"common/notify"
	"common/settings"
	"common/system"
	"common/verifycache"
	"fmt"
	"nvm/bootstrap"
	"nvm/commands"
	"nvm/commands/firewall"
	"nvm/installer"
	"nvm/legacy"
	"nvm/log"
	"os"
	"path/filepath"
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

	// Toast / protocol activation (e.g. nvm://firewall?action=trust&...).
	if strings.HasPrefix(strings.ToLower(os.Args[1]), "nvm://") {
		settings.Load()
		if err := firewall.HandleProtocolURI(os.Args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "--register-eventlog":
		// Invoked by OSS installer to support event log registration without needing to run the entire CLI installer.
		// Runs elevated so it can write to HKLM.
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
	case "--remove-legacy-system-env":
		// Invoked by the certified MSI (elevated) after InstallFiles to clear v1
		// SYSTEM NVM_HOME/NVM_SYMLINK and strip community program-root PATH entries
		// without re-registering the ETW provider (MSI uses wevtutil for that).
		if err := legacy.RemoveSystemEnvVars(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--remove-legacy-user-env":
		// Impersonated MSI CA: clear leftover HKCU NVM_* and community user PATH
		// so shells pick Program Files nvm without waiting for first bootstrap.
		settings.Load()
		dataRoot, err := bootstrap.DataRoot()
		if err != nil || dataRoot == "" {
			local := os.Getenv("LOCALAPPDATA")
			if local == "" {
				fmt.Fprint(os.Stderr, "LOCALAPPDATA is empty; cannot clean user env\n")
				os.Exit(1)
			}
			dataRoot = filepath.Join(local, "Author Software", "nvm")
		}
		if err := bootstrap.RemoveLegacyCurrentUserEnv(dataRoot); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
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
	case "--sign-version-scripts":
		// Invoked by detached reshim after global package installs so proxy
		// can trust newly written .cmd/.bat launchers without executing them first.
		versionDir, wantSignChanged := parseSignVersionScriptsArgs(os.Args[2:])
		if versionDir == "" {
			fmt.Fprint(os.Stderr, "missing version directory for --sign-version-scripts\n")
			os.Exit(1)
		}
		settings.Load()
		_ = os.Unsetenv("NVM_SIGN_CHANGED_MODULES")
		if wantSignChanged && verifycache.ParentIsNvmReshim() {
			verifycache.SetAllowSignChanged(true)
		}
		if err := verifycache.SignVersionScripts(versionDir); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--sign-script":
		// Force-resign one launcher after trust prompt (bypass TrustedModules gate).
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, "missing script path for --sign-script\n")
			os.Exit(1)
		}
		settings.Load()
		if err := verifycache.SignScript(os.Args[2]); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--reshim":
		// Invoked by proxy after global package installs. Must open the .shim
		// ACL write window (RunWithRuntimeShimWrite); spawning reshim.exe alone
		// cannot create hardlinks against the locked directory.
		settings.Load()
		args, readyEvent := splitReshimArgs(os.Args[2:])
		_ = os.Unsetenv("NVM_SIGN_CHANGED_MODULES")
		if verifycache.AuthorizeSignChangedFromParent() {
			verifycache.SetAllowSignChanged(true)
			args = append(args, "--sign-changed")
		}
		system.SignalNamedEvent(readyEvent)
		if err := bootstrap.RunReshim(args...); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--cleanup-user-appdata":
		// Invoked by the MSI during a real uninstall to remove the current user's
		// AppData runtime root before Program Files payload removal.
		if err := installer.CleanupCurrentUserAppData(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--clear-machine-licensing":
		// Invoked by the Inno Setup uninstaller before payload removal.
		if err := installer.ClearMachineLicensing(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--remove-sync-tasks":
		// Invoked by the MSI on install/upgrade so reinstalls drop stale sync tasks
		// (community or prior certified). First-launch bootstrap recreates the task.
		if err := installer.RemoveSyncScheduledTasks(); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--seed-announcement-watermarks":
		// MSI writes HKLM last news/update/sync checks so hourly sync does not
		// toast historical feed items before the user launches nvm.
		settings.Load()
		if err := settings.SeedAnnouncementWatermarksIfEmpty(settings.PutMachine); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--repair-runtime-acls":
		// Invoked by Inno Setup post-install (elevated) and mirrors doctor --autofix ACL repair:
		// harden InstallRoot/DataRoot, repair each installed version dir (NVM4305), re-lock shim/proxy.
		settings.Load()
		installRoot, err := bootstrap.InstallRoot()
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		dataRoot, err := bootstrap.DataRoot()
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		if err := fs.RepairRuntimeACLs(installRoot, dataRoot); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		_ = settings.Put("runtime_acl_degraded", false)
		return
	case "--check-runtime-acls":
		// Exit 0 when InstallRoot (+ version dirs) are trust-safe after a current-token repair attempt.
		// Optional arg: absolute InstallRoot to check (silent installer probes HKCU path before migrating).
		settings.Load()
		installRoot := ""
		if len(os.Args) >= 3 && strings.TrimSpace(os.Args[2]) != "" {
			installRoot = filepath.Clean(settings.Expand(strings.TrimSpace(os.Args[2])))
		} else {
			var err error
			installRoot, err = bootstrap.InstallRoot()
			if err != nil {
				fmt.Fprint(os.Stderr, err.Error())
				os.Exit(1)
			}
		}
		dataRoot := filepath.Dir(installRoot)
		_ = fs.RepairRuntimeACLs(installRoot, dataRoot)
		if fs.AllowsCrossUserWrite(installRoot) {
			fmt.Fprint(os.Stderr, "install root remains writable by other users\n")
			os.Exit(1)
		}
		issues, err := fs.CollectVersionDirectoryTrustIssues(installRoot)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		if len(issues) > 0 {
			fmt.Fprintf(os.Stderr, "%d version director(y/ies) remain untrusted\n", len(issues))
			os.Exit(1)
		}
		os.Exit(0)
		return
	case "--notify-storage-migrated":
		// Silent installer toast after forcing AppData because prior InstallRoot was unsafe.
		if len(os.Args) < 4 {
			fmt.Fprint(os.Stderr, "usage: nvm --notify-storage-migrated <old-root> <new-root>\n")
			os.Exit(1)
		}
		oldRoot := strings.TrimSpace(os.Args[2])
		newRoot := strings.TrimSpace(os.Args[3])
		_ = notify.Register(settings.AppId, eventSourceName)
		msg := fmt.Sprintf(
			"Previous Node.js storage was not permission-safe, so NVM moved it to AppData.\n\nOld: %s\nNew: %s\n\nTo use the old path later (after hardening): nvm cfg set root=%s",
			oldRoot,
			newRoot,
			oldRoot,
		)
		if err := notify.Send(settings.AppId, "NVM storage location updated", msg); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "--unregister-eventlog":
		// Invoked by OSS uninstaller to support event log unregistration without needing to run the entire CLI uninstaller.
		if err := log.UnregisterEventSource(eventSourceName); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(1)
		}
		return
	case "-v", "--version", "version":
		settings.Load()
		fmt.Printf("v%s\n", version)
		warnCommunityProgramRootIfNeeded()
		return
	case "-h", "--help", "help":
		// Handled by kong after lightweight init (no shim/reshim/ARP).
	default:
		if strings.HasPrefix(os.Args[1], "-") {
			fmt.Printf("Unknown flag: %s\n", os.Args[1])
			os.Exit(1)
		}
	}

	root := any(&commands.Root)
	metaHelp := os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help"
	if metaHelp {
		root = &commands.RootWithVersion
	}

	if system.IsProcessStartedByExplorer() && !argsContainFromApps(os.Args) {
		fmt.Printf("%s v%s should be run from a shell/terminal.\nPress 'Enter' to exit...\n", name, version)
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}

	settings.Load()
	settings.ProductVersion = version
	warnCommunityProgramRootIfNeeded()

	desc := fmt.Sprintf("%s\nv%s (%s Edition).", description, version, license.Edition())

	cli := kong.Parse(
		root,
		kong.Name(name),
		kong.Description(desc),
		kong.UsageOnError(),
		kong.Vars{
			"app":       name,
			"version":   version,
			"node":      "Node.js",
			"buildTime": buildTime,
			"cfg_opts":  strings.Join(settings.ListUserCfg(), ", "),
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

	// Bootstrap/hardening only after Kong accepts a real command.
	// Help (--help / bare nvm / `help`) and parse failures exit inside Parse
	// and never reach here — so they skip reshim/ACL/schtasks entirely.
	if commandNeedsBootstrap(cli.Command()) {
		if err := bootstrap.EnsureUserProfileInitialized(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}

	if err := cli.Run(); err != nil {
		// Print error without Kong's "nvm: error: " prefix
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func commandNeedsBootstrap(commandPath string) bool {
	cmd := strings.ToLower(strings.TrimSpace(commandPath))
	if cmd == "" || cmd == "help" {
		return false
	}
	// Proxy firewall helpers run on every npm i. Skip bootstrap here so an MSI
	// overwrite does not pay MaintainShimDirectory sync+reshim on the install hot path.
	// Shim sync still runs on the next user-facing nvm command.
	if strings.HasPrefix(cmd, "firewall check-remote") ||
		strings.HasPrefix(cmd, "firewall refresh-npm-identity") {
		return false
	}
	// First path segment only (e.g. "install <version>" → "install").
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		cmd = cmd[:i]
	}
	switch cmd {
	case "help":
		return false
	default:
		return true
	}
}

func argsContainFromApps(args []string) bool {
	for _, arg := range args {
		if arg == "--from-apps" {
			return true
		}
	}
	return false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func parseSignVersionScriptsArgs(args []string) (versionDir string, signChanged bool) {
	for _, a := range args {
		if a == "--sign-changed" {
			signChanged = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			continue
		}
		if versionDir == "" {
			versionDir = a
		}
	}
	return versionDir, signChanged
}

func splitReshimArgs(args []string) (forward []string, readyEvent string) {
	forward = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--sign-changed" {
			continue
		}
		if a == "--parent-ready-event" {
			if i+1 < len(args) {
				readyEvent = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "--parent-ready-event=") {
			readyEvent = strings.TrimPrefix(a, "--parent-ready-event=")
			continue
		}
		forward = append(forward, a)
	}
	return forward, readyEvent
}
