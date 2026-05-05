package cache

type Add struct {
	Version SaveVersion `cmd:"version" default:"withargs" help:"Cache ${node} downloads without installing. This is run if no subcommand is specified (e.g. nvm cache add)."`
}
