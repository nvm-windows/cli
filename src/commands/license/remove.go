package license

import (
	"common/settings"
	"common/system"
	"fmt"
	"strings"
)

type Clear struct{}

func (s *Clear) Run() error {
	if err := system.RequireAdministrator(); err != nil {
		return err
	}

	for _, item := range []struct {
		name  string
		label string
	}{
		{name: "access_token", label: "access token"},
		{name: "access_key", label: "mirror authentication key"},
	} {
		if err := clearMachineLicensingValue(item.name, item.label); err != nil {
			return err
		}
	}

	return nil
}

func clearMachineLicensingValue(name, label string) error {
	if err := settings.DelMachine(name); err != nil {
		return fmt.Errorf("failed to clear machine %s: %w", label, err)
	}

	got, err := settings.Get(name)
	if err != nil {
		return err
	}

	if stored, ok := got.(string); ok && strings.TrimSpace(stored) != "" {
		return fmt.Errorf("failed to clear machine %s (value still present after delete; run from an elevated shell)", label)
	}

	return nil
}
