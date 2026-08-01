//go:build certified

package build_test

import (
	"testing"

	"common/acl"
)

func TestCertifiedBuildLinksPolicyACL(t *testing.T) {
	if acl.Implementation() != "policy" {
		t.Fatalf("common/acl implementation = %q, want policy (certified builds must replace ../../common/acl with ../../enhanced/go/acl)", acl.Implementation())
	}
}
