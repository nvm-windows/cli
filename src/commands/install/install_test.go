package install

import (
	"reflect"
	"testing"
)

func TestDefaultInstallVersionsEmptyIsLatest(t *testing.T) {
	got := defaultInstallVersions(nil)
	want := []string{"latest"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultInstallVersions(nil) = %#v, want %#v", got, want)
	}

	got = defaultInstallVersions([]string{})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultInstallVersions([]) = %#v, want %#v", got, want)
	}
}

func TestDefaultInstallVersionsPreservesExplicit(t *testing.T) {
	in := []string{"24", "lts", "alpha"}
	got := defaultInstallVersions(in)
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("defaultInstallVersions(%#v) = %#v, want same slice", in, got)
	}
}
