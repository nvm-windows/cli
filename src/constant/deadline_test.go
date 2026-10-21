package constant

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

func TestDeadlineFlagParsing(t *testing.T) {
	parse := func(args ...string) (DeadlineFlags, RelaxDeadlines, []string, error) {
		t.Helper()
		var cli struct {
			RelaxDeadlines RelaxDeadlines `optional:"" placeholder:"MS"`
			DeadlineFlags  `embed:""`
			Version        []string `arg:"" optional:""`
		}
		parser := kong.Must(&cli)
		_, err := parser.Parse(args)
		return cli.DeadlineFlags, cli.RelaxDeadlines, cli.Version, err
	}

	mirror, relax, version, err := parse("--timeout-catalog-mirror-ms", "3000", "22")
	if err != nil {
		t.Fatal(err)
	}
	if !mirror.TimeoutCatalogMirrorMs.Set() || mirror.TimeoutCatalogMirrorMs.Milliseconds() != 3000 {
		t.Fatalf("mirror = %+v", mirror.TimeoutCatalogMirrorMs)
	}
	if mirror.TimeoutCatalogMs.Set() || relax.Active() {
		t.Fatalf("other flags set: %+v %+v", mirror, relax)
	}
	if len(version) != 1 || version[0] != "22" {
		t.Fatalf("version = %v", version)
	}
	steps := mirror.Setting()
	if steps.CatalogMirrorMs != 3000 || steps.CatalogMs != 0 || steps.DownloadMs != 0 {
		t.Fatalf("steps = %+v", steps)
	}
	if got := strings.Join(mirror.Args(), " "); got != "--timeout-catalog-mirror-ms=3000" {
		t.Fatalf("args = %q", got)
	}

	both, relax, version, err := parse("--relax-deadlines", "--timeout-download-ms=5000", "20")
	if err != nil {
		t.Fatal(err)
	}
	if !relax.Active() || relax.Milliseconds() != 0 {
		t.Fatalf("relax = %+v", relax)
	}
	if both.TimeoutDownloadMs.Milliseconds() != 5000 || both.Setting().DownloadMs != 5000 {
		t.Fatalf("download = %+v", both.TimeoutDownloadMs)
	}
	if len(version) != 1 || version[0] != "20" {
		t.Fatalf("version = %v", version)
	}

	if _, _, _, err := parse("--timeout-catalog-ms=0"); err == nil {
		t.Fatal("expected rejection of 0")
	}
	if _, _, _, err := parse("--timeout-reachability-ms"); err == nil {
		t.Fatal("expected rejection of a missing value")
	}
}
