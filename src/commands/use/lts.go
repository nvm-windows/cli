package use

type LTSAlias struct {
	Alias string `arg:"alias" optional:"" help:"The LTS alias to use (e.g. iron, fermium)."`
}

func (c *LTSAlias) Run() error {
	var cmd Version
	if c.Alias == "" || c.Alias == "latest" {
		cmd.Version = []string{"lts"}
	} else {
		cmd.Version = []string{"lts/" + c.Alias}
	}

	return cmd.Run()
}
