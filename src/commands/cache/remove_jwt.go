package cache

import (
	"fmt"
	"os"

	"common/mirrorauth"
)

type RemoveJWT struct{}

func (c *RemoveJWT) Run() error {
	if mirrorauth.ClearCachedLicenseJWT() {
		fmt.Fprintln(os.Stdout, "cleared mirror license JWT cache.")
		return nil
	}

	fmt.Fprintln(os.Stdout, "no mirror license JWT cache to clear.")
	return nil
}
