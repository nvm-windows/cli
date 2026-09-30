package firewall

import (
	"common/modulefirewall"
	"reflect"
	"strings"
	"testing"
)

func TestDedupeList(t *testing.T) {
	got := dedupeList([]string{"eslint", "ESLint", " porthog ", "porthog", "", "NOT ALL", "not all"})
	want := []string{"eslint", "porthog", "NOT ALL"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestEntryMatchesRemove(t *testing.T) {
	tests := []struct {
		stored, want string
		match        bool
	}{
		{"porthog", "porthog", true},
		{"Porthog", "porthog", true},
		{"NOT porthog", "porthog", true},
		{"!porthog", "porthog", true},
		{"NOT porthog", "NOT porthog", true},
		{"!porthog", "NOT porthog", true},
		{"eslint", "porthog", false},
		{"NOT ALL", "ALL", true},
		{"ALL", "NOT ALL", true},
	}
	for _, tt := range tests {
		if got := entryMatchesRemove(tt.stored, tt.want); got != tt.match {
			t.Fatalf("entryMatchesRemove(%q,%q)=%v, want %v", tt.stored, tt.want, got, tt.match)
		}
	}
}

func TestStripNegation(t *testing.T) {
	if got := stripNegation("NOT eslint"); got != "eslint" {
		t.Fatalf("got %q", got)
	}
	if got := stripNegation("!eslint"); got != "eslint" {
		t.Fatalf("got %q", got)
	}
	if got := stripNegation("eslint"); got != "eslint" {
		t.Fatalf("got %q", got)
	}
}

func TestRemoteFailureText(t *testing.T) {
	url := "https://127.0.0.1:8443/module/trust"
	text, code, exitCode := remoteFailureText(modulefirewall.RemoteResult{Unreachable: true}, url, "machine policy")
	if text != "cannot reach https://127.0.0.1:8443/module/trust (enforced by machine policy)." || code != CodeRemoteUnreachable || exitCode != 2 {
		t.Fatalf("unreachable text=%q code=%d exit=%d", text, code, exitCode)
	}

	text, code, exitCode = remoteFailureText(modulefirewall.RemoteResult{Status: 403}, url, "machine settings")
	if text != "blocked by machine settings." || code != CodeModuleBlocked || exitCode != 1 {
		t.Fatalf("blocked text=%q code=%d exit=%d", text, code, exitCode)
	}

	text, code, exitCode = remoteFailureText(modulefirewall.RemoteResult{Status: 401}, url, "your settings")
	if text != "blocked by your settings." || code != CodeRemoteUnauthorized || exitCode != 1 {
		t.Fatalf("unauthorized text=%q code=%d exit=%d", text, code, exitCode)
	}
	if strings.Contains(text, "https://") || strings.Contains(text, "127.0.0.1") {
		t.Fatalf("working authority leaked url: %q", text)
	}
}
