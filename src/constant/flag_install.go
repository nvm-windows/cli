package constant

type FlagInstall struct {
	Install bool `flag:"install" short:"i" help:"Automatically install the version if it's not already installed. Applies only when auto-installation is disabled."`
}

type FlagNoInstall struct {
	NoInstall bool `flag:"no-install" short:"n" help:"Do not automatically install the version if it's not already installed. Applies only when auto-installation is enabled."`
}
