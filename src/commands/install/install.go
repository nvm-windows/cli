package install

import (
	"common/settings"
	"nvm/commands/cache"
	"nvm/constant"
	"nvm/installer"

	"github.com/alecthomas/kong"
)

type Install struct {
	CopyFrom string `flag:"copy-from" help:"Copy the global modules from an existing Node.js installation to the new installation. This does affect the original version and does not verify module compatibility with the new version." placeholder:"VERSION"`
	From     string `flag:"from" help:"Install global modules from the list of modules installed in an another Node.js version." placeholder:"VERSION"`
	Notify   bool   `flag:"notify" hidden:"true" help:"Notify user when a version has been automatically installed."`
	Cache    bool   `flag:"cache" help:"Save the download (offline cache)."`
	NoCache  bool   `flag:"no-cache" help:"Install without caching. This is the default unless configured to always cache downloads."`
	Force    bool   `flag:"force" help:"Force (re)install if the version already exists."`
	Debug    bool   `flag:"debug" hidden:"true" help:"Write timing logs to the install directory for debugging."`
	Insecure bool   `flag:"insecure" help:"Accept invalid TLS/SSL certs from download sources."`
	constant.ArgVersion
}

func (s *Install) Run(ctx *kong.Context) error {
	cfg := settings.Global()

	var cacheRoot string
	if cfg.LocalInstallDir != "" {
		cacheRoot = cfg.LocalInstallDir
		// fmt.Printf("using local install dir as cache: %s\n", cacheRoot)
	} else {
		cacheRoot = cache.Store.Versions
	}

	return installer.Install(installer.InstallConfig{
		Notify:                s.Notify,
		Debug:                 s.Debug,
		Cache:                 s.Cache,
		NoCache:               s.NoCache,
		Force:                 s.Force,
		CacheDir:              cacheRoot,
		Versions:              s.Version,
		AllowInsecure:         s.Insecure,
		CopyModulesFrom:       s.CopyFrom,
		ModulesFrom:           s.From,
		AutoInstallModuleList: cfg.AutoInstallModuleList,
		LocalOnly:             cfg.LocalInstallOnly,
	})
}
