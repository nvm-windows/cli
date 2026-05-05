package cache

import (
	"nvm/constant"
	"nvm/installer"
)

// Install is the cache-local equivalent of commands.Install with SaveOnly
// forced to true. It lives here (rather than in the parent commands package)
// to avoid an import cycle: commands → commands/cache → commands.
type SaveVersion struct {
	constant.ArgVersion
	Insecure bool `flag:"insecure" help:"Accept invalid SSL certificates from download sources."`
}

func (s *SaveVersion) Run() error {
	return installer.Install(installer.InstallConfig{
		Cache:         true,
		CacheOnly:     true,
		NoCache:       false,
		Force:         false,
		AllowInsecure: s.Insecure,
		CacheDir:      Store.Versions,
		Versions:      s.Version,
	})
}
