package clitest

import (
	"common/settings"
	"nvm/commands"
	"strings"

	"github.com/alecthomas/kong"
)

// ExecuteOptions configures Execute.
type ExecuteOptions struct {
	// Bootstrap runs EnsureUserProfileInitialized before the command.
	Bootstrap bool
}

// Execute parses and runs nvm subcommands through Kong against the sandbox profile.
func (s *Sandbox) Execute(args ...string) (stdout, stderr string, err error) {
	return s.ExecuteWithOptions(ExecuteOptions{}, args...)
}

// ExecuteWithOptions parses and runs nvm subcommands with optional bootstrap.
func (s *Sandbox) ExecuteWithOptions(opts ExecuteOptions, args ...string) (stdout, stderr string, err error) {
	s.t.Helper()

	settings.Load(true)

	if opts.Bootstrap {
		if err := ensureBootstrap(s); err != nil {
			return "", "", err
		}
	}

	parser, err := kong.New(
		&commands.Root,
		kong.Name("nvm"),
		kong.Description("nvm test harness"),
		kong.UsageOnError(),
		kong.Vars{
			"app":      "nvm",
			"node":     "Node.js",
			"cfg_opts": strings.Join(settings.ListUserCfg(), ", "),
		},
	)
	if err != nil {
		return "", "", err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		return "", "", err
	}

	out, runErr := CaptureOutput(func() error {
		return ctx.Run()
	})
	return out.Stdout, out.Stderr, runErr
}
