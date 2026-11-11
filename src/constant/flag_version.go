package constant

type ArgVersion struct {
	Version []string `arg:"version" required:"" help:"The version of Node.js to install (e.g. latest, alpha, lts, lts/iron, x.x.x, x.x, x). Omitting the version on nvm install installs latest (stable)."`
}

type ArgVersionOptional struct {
	Version []string `arg:"version" optional:"" help:"The version of Node.js to install (e.g. latest, alpha, lts, lts/iron, x.x.x, x.x, x). Omit to install latest (stable)."`
}
