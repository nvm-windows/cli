package constant

import (
	"common/settings"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

// DeadlineMs is one optional millisecond flag.
// Fields stay unexported so kong treats the value as one flag.
type DeadlineMs struct {
	set bool
	ms  int
}

func (d DeadlineMs) Set() bool { return d.set }

func (d DeadlineMs) Milliseconds() int { return d.ms }

func (d *DeadlineMs) Decode(ctx *kong.DecodeContext) error {
	raw, ok := tokenString(ctx.Scan.Peek().Value)
	if !ok || raw == "" || strings.HasPrefix(raw, "-") {
		return fmt.Errorf("must be a positive number of milliseconds")
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fmt.Errorf("must be a positive number of milliseconds")
	}
	ctx.Scan.Pop()
	d.set = true
	d.ms = n
	return nil
}

// DeadlineFlags sets one network deadline for this command.
// Kong embed flattens these fields onto the command.
type DeadlineFlags struct {
	TimeoutCatalogMs       DeadlineMs `optional:"" placeholder:"MS" help:"Set the catalog deadline for this command, in milliseconds."`
	TimeoutCatalogMirrorMs DeadlineMs `optional:"" placeholder:"MS" help:"Set the per-mirror catalog deadline for this command, in milliseconds."`
	TimeoutReachabilityMs  DeadlineMs `optional:"" placeholder:"MS" help:"Set the reachability deadline for this command, in milliseconds."`
	TimeoutDownloadMs      DeadlineMs `optional:"" placeholder:"MS" help:"Set the download deadline for this command, in milliseconds."`
}

func (f DeadlineFlags) Setting() settings.StepDeadlines {
	return settings.StepDeadlines{
		CatalogMs:       f.TimeoutCatalogMs.Milliseconds(),
		CatalogMirrorMs: f.TimeoutCatalogMirrorMs.Milliseconds(),
		ReachabilityMs:  f.TimeoutReachabilityMs.Milliseconds(),
		DownloadMs:      f.TimeoutDownloadMs.Milliseconds(),
	}
}

func (f DeadlineFlags) Args() []string {
	args := make([]string, 0, 4)
	add := func(flag string, d DeadlineMs) {
		if d.Set() {
			args = append(args, fmt.Sprintf("--%s=%d", flag, d.Milliseconds()))
		}
	}
	add("timeout-catalog-ms", f.TimeoutCatalogMs)
	add("timeout-catalog-mirror-ms", f.TimeoutCatalogMirrorMs)
	add("timeout-reachability-ms", f.TimeoutReachabilityMs)
	add("timeout-download-ms", f.TimeoutDownloadMs)
	return args
}
