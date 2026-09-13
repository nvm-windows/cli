package license

import (
	nvmlicense "common/license"
	"common/settings"
	"common/system"
	"common/token"
	"fmt"
	"strings"
	"time"
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

	if name == "access_token" {
		if err := token.Set(trimmed); err != nil {
			return fmt.Errorf("failed to verify access token: %w", err)
		}
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

	if name == "access_token" {
		// Provisional stamp so deploy isn't Community until the first sync tick.
		if err := nvmlicense.StampLicenseVerified(time.Now()); err != nil {
			return fmt.Errorf("access token stored but failed to stamp verification time: %w", err)
		}
	}

	return nil
}
