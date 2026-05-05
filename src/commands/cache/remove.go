package cache

type Remove struct {
	All      Clear          `cmd:"all" help:"Clear all caches."`
	Metadata RemoveMetadata `cmd:"metadata" help:"Remove cached metadata."`
	Version  RemoveVersion  `cmd:"version" default:"withargs" help:"Remove cached ${node} downloads. This command runs when no subcommand is specified (i.e. nvm cache rm)"`
}
