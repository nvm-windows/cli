package license

import (
	"common/registry"
	"common/system"
	"fmt"
	"strings"
)

var org string

type Set struct {
	Key string `arg:"" name:"key" help:"The license key to set for this installation." required:""`
}

func (s *Set) Run() error {
	if err := system.RequireAdministrator(); err != nil {
		return err
	}

	regpath := "HKLM/SOFTWARE/" + org + "/nvm/License"
	if err := registry.Put([]byte(s.Key), regpath); err != nil {
		return err
	}

	value, exists, err := registry.Get(regpath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("failed to set machine license at %s (value not found after write; run from an elevated shell)", regpath)
	}

	stored, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to set machine license at %s (unexpected registry value type %T)", regpath, value)
	}

	if string(stored) != strings.TrimSpace(s.Key) {
		return fmt.Errorf("failed to verify machine license at %s after write", regpath)
	}

	return nil
}
