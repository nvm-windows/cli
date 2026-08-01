package license

import (
	"common/settings"
	"common/system"
	"fmt"
	"strings"
)

type SetKey struct {
	Value string `arg:"" help:"The mirror authentication key to store for this installation." required:""`
}

func (s *SetKey) Run() error {
	return setMachineLicensingValue("access_key", "mirror authentication key", s.Value)
}

func setMachineLicensingValue(name, label, value string) error {
	if err := system.RequireAdministrator(); err != nil {
		return err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s is empty", label)
	}

	if err := settings.PutMachine(name, trimmed); err != nil {
		return fmt.Errorf("failed to set machine %s: %w", label, err)
	}

	got, err := settings.Get(name)
	if err != nil {
		return err
	}

	stored, ok := got.(string)
	if !ok || stored != trimmed {
		return fmt.Errorf("failed to verify machine %s after write", label)
	}

	return nil
}
