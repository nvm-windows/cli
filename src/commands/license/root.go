package license

type Root struct {
	Set   Set   `cmd:"set" help:"Set or update the ${app} license key."`
	Clear Clear `cmd:"clear" aliases:"rm" help:"Clear the ${app} license key from this installation."`
}
