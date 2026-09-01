package installer

import "testing"

func TestHighestSemverVersion(t *testing.T) {
	got := highestSemverVersion([]string{"", "20.11.0", "22.23.2", "22.1.0"})
	if got != "22.23.2" {
		t.Fatalf("highestSemverVersion() = %q, want 22.23.2", got)
	}
}

func TestHighestSemverVersionEmpty(t *testing.T) {
	if got := highestSemverVersion(nil); got != "" {
		t.Fatalf("highestSemverVersion(nil) = %q, want empty", got)
	}
}
