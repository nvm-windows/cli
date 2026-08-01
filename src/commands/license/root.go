package license

type Root struct {
	Set   SetRoot `cmd:"set" help:"Set Author licensing values on this machine."`
	Clear Clear   `cmd:"clear" aliases:"rm" help:"Clear the ${app} license key from this installation."`
}
