//go:build certified

package build_test

import (
	"testing"

	"common/mirrorauth"
)

func TestCertifiedBuildLinksMirrorAuth(t *testing.T) {
	if mirrorauth.Implementation() != "certified" {
		t.Fatalf("common/mirrorauth implementation = %q, want certified", mirrorauth.Implementation())
	}
}
