package list

type Root struct {
	Installed Installs `cmd:"installed" default:"withargs" help:"List the installed ${node} versions. This is run if no subcommand is specified (e.g. nvm ls)."`
	Releases  Releases `cmd:"releases" help:"List the downloadable ${node} versions."`
	Available Releases `cmd:"available" hidden:"true" help:"Alias for 'released'."`
	Cached    Cache    `cmd:"cached" help:"List the cached ${node} versions."`
}
