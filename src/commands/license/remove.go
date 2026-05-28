package license

import (
	"common/registry"
	"common/system"
	"fmt"
)

type Clear struct{}

func (s *Clear) Run() error {
	if err := system.RequireAdministrator(); err != nil {
		return err
	}

	regpath := "HKLM/Software/" + org + "/nvm/License"
	if err := registry.Del(regpath); err != nil {
		return err
	}

	_, exists, err := registry.Get(regpath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("failed to clear machine license at %s (value still present after delete; run from an elevated shell)", regpath)
	}

	return nil
}
