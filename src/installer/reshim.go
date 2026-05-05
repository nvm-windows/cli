package installer

import (
	proxy "nvm/reshim"
)

func reshim() error {
	return proxy.Run()
}
