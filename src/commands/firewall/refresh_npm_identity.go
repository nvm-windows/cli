package firewall

import (
	"common/modulefirewall"
	"common/settings"
	"os"
)

// RefreshNpmIdentity refreshes the local npm login identity cache after
// npm/pnpm login, adduser, logout, or whoami (proxy-invoked).
// When a credential is present but username is unknown, captures via `npm whoami`
// once (network allowed only on this path — not during install JWT mint).
type RefreshNpmIdentity struct{}

func (c *RefreshNpmIdentity) Run() error {
	cfg := settings.Global()
	cwd, _ := os.Getwd()
	_ = modulefirewall.CaptureNpmIdentity(cwd, settings.Expand(cfg.Root), cfg.ActiveVersion)
	return nil
}
