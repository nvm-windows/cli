//go:build !windows

package bootstrap

const (
	CommunityAppxName = "AuthorSoftware.NVMWindows"
	CertifiedAppxName = "AuthorSoftware.NVMWindows.Certified"
)

func RegisterSparseIdentity(string) error { return nil }

func UnregisterSparseIdentity(string) error { return nil }

func MaybeRemoveStaleCommunityIdentity() {}
