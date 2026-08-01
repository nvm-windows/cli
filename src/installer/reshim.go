package installer

import "nvm/bootstrap"

func reshim() error {
	return bootstrap.RunReshim("--silent")
}
