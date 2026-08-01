package license

type SetToken struct {
	Value string `arg:"" help:"The access token to store for this installation." required:""`
}

func (s *SetToken) Run() error {
	return setMachineLicensingValue("access_token", "access token", s.Value)
}
