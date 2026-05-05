package constant

type FlagLimits struct {
	Limit   int  `flag:"limit" short:"l" default:"20" help:"Maximum number of versions to list (can be very long)."`
	NoLimit bool `flag:"no-limit" help:"List all versions without truncating."`
}
