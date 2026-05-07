package commands

import (
	"nvm/commands/cache"
	"nvm/constant"
	"nvm/installer"
)

type Uninstall struct {
	Purge  bool `flag:"purge" help:"Purge the cache of this version (if cached)."`
	Notify bool `flag:"notify" hidden:"true" help:"Quietly notify user when a version has been automatically installed."`
	constant.ArgVersion
}

func (s *Uninstall) Run() error {
	return installer.Uninstall(installer.UninstallConfig{
		Versions:   s.Version,
		ClearCache: s.Purge,
		CacheDir:   cache.Store.Versions,
	})
}
