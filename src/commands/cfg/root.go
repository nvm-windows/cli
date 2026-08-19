package cfg

type Root struct {
	List  List `cmd:"list" aliases:"ls" default:"withargs" help:"List all ${app} configuration values."`
	Get   Get  `cmd:"get" help:"Get ${app} configuration value(s)."`
	Set   Set  `cmd:"set" help:"Set ${app} configuration value(s)."`
	Reset ResetRoot `cmd:"reset" aliases:"rm" help:"Reset ${app} configuration to defaults."`
	Docs  Docs `cmd:"docs" help:"Explanations of each ${app} configuration option."`
}
