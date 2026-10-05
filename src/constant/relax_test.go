package constant

import (
	"testing"

	"github.com/alecthomas/kong"
)

func TestRelaxFlagParsing(t *testing.T) {
	parse := func(args ...string) (RelaxDeadlines, []string) {
		t.Helper()
		var cli struct {
			RelaxDeadlines RelaxDeadlines `optional:"" placeholder:"MS" help:"Relax network deadlines for this command."`
			Version        []string       `arg:"" optional:""`
		}
		parser := kong.Must(&cli)
		_, err := parser.Parse(args)
		if err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		return cli.RelaxDeadlines, cli.Version
	}

	bare, version := parse("--relax-deadlines")
	if !bare.Active() || bare.Milliseconds() != 0 {
		t.Fatalf("bare = %+v", bare)
	}
	if len(version) != 0 {
		t.Fatalf("bare consumed args: %v", version)
	}

	custom, version := parse("--relax-deadlines", "9000", "20")
	if !custom.Active() || custom.Milliseconds() != 9000 {
		t.Fatalf("custom = %+v", custom)
	}
	if len(version) != 1 || version[0] != "20" {
		t.Fatalf("version = %v", version)
	}

	absent, version := parse("22")
	if absent.Active() {
		t.Fatal("flag should be absent")
	}
	if len(version) != 1 || version[0] != "22" {
		t.Fatalf("version = %v", version)
	}
}
