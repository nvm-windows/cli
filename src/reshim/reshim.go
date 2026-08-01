package reshim

import "nvm/bootstrap"

func Run() error {
	return bootstrap.RunReshim("--silent")
}
