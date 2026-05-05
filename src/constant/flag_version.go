package constant

type ArgVersion struct {
	Version []string `arg:"version" required:"" help:"The version of Node.js to install (e.g. latest, lts, lts/iron, x.x.x, x.x, x)."`
}

type ArgVersionOptional struct {
	Version []string `arg:"version" optional:"" help:"The version of Node.js to install (e.g. latest, lts, lts/iron, x.x.x, x.x, x)."`
}
