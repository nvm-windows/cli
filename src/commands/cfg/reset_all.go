package cfg

import (
	"common/settings"
	"fmt"
	"os"
	"strings"
)

type ResetRoot struct {
	All    ResetAll `cmd:"all" help:"Reset all configuration options to defaults except root."`
	Option Del      `cmd:"option" default:"withargs" help:"Reset one configuration option to its default (e.g. nvm config reset cache_downloads)."`
}

type ResetAll struct {
	Quiet bool `flag:"quiet" short:"q" help:"Suppress non-essential output."`
}

func (r *ResetAll) Run() error {
	return resetAllExceptRoot(r.Quiet)
}

func resetAllExceptRoot(quiet bool) error {
	reset := make([]string, 0)
	skipped := make([]string, 0)

	for _, name := range settings.List() {
		if settings.IsExcludedFromBulkReset(name) {
			continue
		}

		if err := resetSetting(name, false); err != nil {
			if settings.IsPolicyManagedError(err) {
				skipped = append(skipped, name)
				continue
			}
			return err
		}

		reset = append(reset, name)
	}

	settings.Load(true)

	if len(reset) == 0 && len(skipped) == 0 {
		if !quiet {
			fmt.Fprintln(os.Stdout, "No configuration options were reset.")
		}
		return nil
	}

	if !quiet {
		if len(reset) > 0 {
			fmt.Fprintf(os.Stdout, "Reset %d configuration option(s) to default.\n", len(reset))
		}
		if len(skipped) > 0 {
			fmt.Fprintf(os.Stdout, "Skipped %d policy-managed option(s): %s\n", len(skipped), strings.Join(skipped, ", "))
		}
	}

	return nil
}
