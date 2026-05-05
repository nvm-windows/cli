package constant

type ArgMajors struct {
	Majors []string `arg:"majors" optional:"" help:"Filter by specific major versions (e.g., 18, 20)."`
}
