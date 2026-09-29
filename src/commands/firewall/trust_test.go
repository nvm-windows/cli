package firewall

import (
	"reflect"
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
