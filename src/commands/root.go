package commands

import (
	"nvm/commands/alias"
	"nvm/commands/cache"
	"nvm/commands/cfg"
	"nvm/commands/install"
	"nvm/commands/list"
	"nvm/commands/use"

	"github.com/alecthomas/kong"
)

type RootCommand struct {
	Install      install.Root  `cmd:"install" aliases:"i,add" help:"Install one or more ${node} versions."`
	InstallTypos install.Root  `cmd:"" aliases:"add,in,ins,inst,insta,instal,isnt,isnta,isntal,isntall" hidden:"true"`
	Uninstall    Uninstall     `cmd:"uninstall" aliases:"rm,un" help:"Uninstall one or more ${node} versions."`
	Use          use.Root      `cmd:"use" help:"Switch the default ${node} version."`
	RC           RunCommand    `cmd:"rc" help:"(Re)create run command file (ex: .nvmrc)."`
	List         list.Root     `cmd:"list" aliases:"ls" help:"List the installed/available ${node} versions."`
	ListRemote   list.Releases `cmd:"list-remote" aliases:"ls-remote" hidden:"true"`
	Alias        alias.Root    `cmd:"alias" help:"Manage ${node} version aliases."`
	Default      Default       `cmd:"default" help:"Show the default version."`
	Current      Default       `cmd:"current" hidden:"true"`
	Env          Env           `cmd:"env" help:"Display ${app} environment details."`
	Cache        cache.Root    `cmd:"cache" help:"View and manage the ${app} cache."`
	Config       cfg.Root      `cmd:"config" aliases:"cfg" help:"View and manage the ${app} configuration."`
	On           Toggle        `cmd:"on" help:"Manage Node.js with ${app}."`
	Off          Toggle        `cmd:"off" help:"Stop managing Node.js with ${app}."`
	Doctor       Doctor        `cmd:"doctor" help:"Detect and fix common ${app} issues." hidden:"true"`
	Debug        Doctor        `cmd:"debug" hidden:"true"`
	Reshim       Reshim        `cmd:"reshim" hidden:"true"`
	Upgrade      Upgrade       `cmd:"upgrade" help:"Upgrade ${app}." hidden:"true"`
}

type RootCommandWithVersion struct {
	RootCommand
	Version kong.VersionFlag `name:"version" short:"v" help:"Display the version of ${app}."`
}

var Root RootCommand
var RootWithVersion RootCommandWithVersion
