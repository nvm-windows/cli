package commands

import (
	"nvm/commands/cache"
	"nvm/constant"
	"nvm/installer"
)

type Uninstall struct {
	Purge  bool `flag:"purge" help:"Purge the cache of this version (if cached)."`
	All    bool `flag:"all" help:"Treat partial versions as a range and uninstall all matching installed versions (e.g. 22 -> all 22.x.x)."`
	Notify bool `flag:"notify" hidden:"true" help:"Quietly notify user when a version has been automatically installed."`
	constant.ArgVersion
}

func (s *Uninstall) Run() error {
	return installer.Uninstall(installer.UninstallConfig{
		Versions:   s.Version,
		ClearCache: s.Purge,
		Range:      s.All,
		CacheDir:   cache.Store.Versions,
	})
}
