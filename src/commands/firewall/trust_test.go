package firewall

import (
	"reflect"
	"testing"
)

func TestCodeConstants(t *testing.T) {
	if CodeRemoteFailed != 4402 {
		t.Fatalf("CodeRemoteFailed=%d, want 4402", CodeRemoteFailed)
	}
	if CodeModuleBlocked != 4403 {
		t.Fatalf("CodeModuleBlocked=%d, want 4403", CodeModuleBlocked)
	}
	if CodeRemoteAllowed != 4407 {
		t.Fatalf("CodeRemoteAllowed=%d, want 4407", CodeRemoteAllowed)
	}
}

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

func TestReadTrustedAtRootAbsent(t *testing.T) {
	list, ok := readTrustedAtRoot("")
	if ok || list != nil {
		t.Fatalf("empty root: list=%v ok=%v", list, ok)
	}
}
