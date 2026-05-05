package alias

type Root struct {
	Add    Add    `cmd:"add" help:"Add or update a ${node} version alias. This is run if no subcommand is specified (e.g. nvm alias)." default:"withargs"`
	List   List   `cmd:"list" aliases:"ls" help:"List aliases."`
	Remove Remove `cmd:"remove" aliases:"rm" help:"Remove an alias."`
}
