package use

type Root struct {
	Version Version     `cmd:"version" default:"withargs" help:"Set the default version. This is the command run when no subcommand is provided (i.e. ${app} use <version>)."`
	LTS     LTSAlias    `cmd:"lts" help:"Use the most recent LTS release version."`
	Latest  LatestAlias `cmd:"latest" help:"Use the latest installed semantic version."`
	Last    Last        `cmd:"last" help:"Use the prior default version."`
	Shim    Mode        `cmd:"shim" help:"Use the shim operating mode."`
	Link    Mode        `cmd:"link" help:"Use the link operating mode."`
}
