package firewall

import (
	"common/modulefirewall"
	"strings"
	"testing"
)

func TestCodeConstants(t *testing.T) {
	if CodeRemoteUnauthorized != 4401 {
		t.Fatalf("CodeRemoteUnauthorized=%d, want 4401", CodeRemoteUnauthorized)
	}
	if CodeRemoteFailed != 4402 {
		t.Fatalf("CodeRemoteFailed=%d, want 4402", CodeRemoteFailed)
	}
	if CodeModuleBlocked != 4403 {
		t.Fatalf("CodeModuleBlocked=%d, want 4403", CodeModuleBlocked)
	}
	if CodeInvalidRule != 4405 {
		t.Fatalf("CodeInvalidRule=%d, want 4405", CodeInvalidRule)
	}
	if CodeRemoteAllowed != 4407 {
		t.Fatalf("CodeRemoteAllowed=%d, want 4407", CodeRemoteAllowed)
	}
	if CodeModuleAllowed != 4408 {
		t.Fatalf("CodeModuleAllowed=%d, want 4408", CodeModuleAllowed)
	}
	if CodeRemoteUnreachable != 4409 {
		t.Fatalf("CodeRemoteUnreachable=%d, want 4409", CodeRemoteUnreachable)
	}
	if CodePolicyMutate != 4410 {
		t.Fatalf("CodePolicyMutate=%d, want 4410", CodePolicyMutate)
	}
}

func TestValidateRuleEntry_CLIPaths(t *testing.T) {
	tests := []struct {
		name    string
		entry   string
		wantErr bool
		errSub  string
	}{
		{"allow ALL", "ALL", false, ""},
		{"deny NOT ALL", "NOT ALL", false, ""},
		{"module", "eslint", false, ""},
		{"org", "@org/*", false, ""},
		{"version pin", "eslint@1.*", false, ""},
		{"https", "https://policy.example/fw", false, ""},
		{"http reject", "http://evil.example/fw", true, "HTTPS"},
		{"empty", "", true, ""},
		{"bang alone", "!", true, ""},
		{"bad org", "org/*", true, "invalid org wildcard"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := modulefirewall.ValidateRuleEntry(tt.entry)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
					t.Fatalf("err=%v, want %q", err, tt.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err=%v", err)
			}
		})
	}
}
