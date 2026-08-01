package clitest

import "nvm/bootstrap"

func ensureBootstrap(s *Sandbox) error {
	return bootstrap.EnsureUserProfileInitialized()
}
