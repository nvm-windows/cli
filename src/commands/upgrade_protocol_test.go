package commands

import "testing"

func TestParseUpgradeProtocolVersion(t *testing.T) {
	tests := []struct {
		in, want string
		ok       bool
	}{
		{"nvm://upgrade?version=2.1.0", "2.1.0", true},
		{"nvm://upgrade?version=v2.1.0-beta.4", "2.1.0-beta.4", true},
		{`"nvm://upgrade?version=2.0.1-hotfix.2"`, "2.0.1-hotfix.2", true},
		{"nvm://firewall?action=trust", "", false},
		{"nvm://upgrade", "", false},
		{"https://example.com", "", false},
	}
	for _, tt := range tests {
		got, err := parseUpgradeProtocolVersion(tt.in)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Fatalf("parseUpgradeProtocolVersion(%q) = (%q, %v), want %q", tt.in, got, err, tt.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("parseUpgradeProtocolVersion(%q) = %q, want error", tt.in, got)
		}
	}
}
