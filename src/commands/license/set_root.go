package license

type SetRoot struct {
	Token SetToken `cmd:"token" default:"withargs" help:"Set the Author access token for this machine (HKLM, REG_BINARY). This runs when no subcommand is provided (i.e. ${app} license set <token>)."`
	Key   SetKey   `cmd:"key" help:"Set the Author mirror authentication key for this machine (HKLM, REG_BINARY)."`
}
