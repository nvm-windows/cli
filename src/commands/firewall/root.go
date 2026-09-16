package firewall

import (
	"common/modulefirewall"
	"common/settings"
	"common/system"
	"fmt"
	"nvm/log"
	"strings"
)

// Event codes (NVM44xx firewall range).
const (
	CodePolicyMutate      = 4401
	CodeElevationRequired = 4404
	CodeInvalidRule       = 4405
)

type Root struct {
	Trust       TrustRoot   `cmd:"trust" help:"Manage TrustedModules for self-updating global CLIs."`
	PromptTrust PromptTrust `cmd:"prompt-trust" hidden:"true" help:"Internal: dual-channel trust prompt for proxy."`
}

type TrustRoot struct {
	Module TrustModule `cmd:"module" help:"Manage TrustedModules (self-update auto-reshim allow list)."`
}

type TrustModule struct {
	Entries []string `arg:"" name:"entry" help:"Module patterns to trust for auto-reshim (or NOT <entry> to revoke)."`
}

func requireMachine() error {
	if err := system.RequireAdministrator(); err != nil {
		log.ErrorStructured("firewall.elevation_required", map[string]any{
			"error": err.Error(),
		}, CodeElevationRequired)
		return fmt.Errorf("firewall policy changes require an elevated administrator prompt (NVM%d): %w", CodeElevationRequired, err)
	}
	return nil
}

func appendSettingsList(cfgKey, regLabel string, add []string, negate bool) error {
	if err := requireMachine(); err != nil {
		return err
	}
	cur, _ := settings.Get(cfgKey)
	var list []string
	switch v := cur.(type) {
	case []string:
		list = append([]string{}, v...)
	case string:
		if strings.TrimSpace(v) != "" {
			list = []string{v}
		}
	}
	for _, e := range add {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if negate && !strings.HasPrefix(strings.ToLower(e), "not ") && !strings.HasPrefix(e, "!") {
			e = "NOT " + e
		}
		if err := modulefirewall.ValidateRuleEntry(e); err != nil {
			log.ErrorStructured("firewall.invalid_rule", map[string]any{
				"key":   cfgKey,
				"entry": e,
				"error": err.Error(),
			}, CodeInvalidRule)
			return fmt.Errorf("invalid firewall entry %q (NVM%d): %w", e, CodeInvalidRule, err)
		}
		list = append(list, e)
	}
	if err := settings.PutMachine(cfgKey, list); err != nil {
		log.ErrorStructured("firewall.policy_mutate_failed", map[string]any{
			"key":   cfgKey,
			"error": err.Error(),
		}, CodePolicyMutate)
		return err
	}
	log.Logf("firewall: updated %s (+%d entries)", regLabel, len(add))
	log.LogStructured("firewall.policy_mutated", map[string]any{
		"key":    cfgKey,
		"added":  add,
		"negate": negate,
		"action": "append",
	}, CodePolicyMutate)
	return nil
}

func (t *TrustModule) Run() error {
	return appendSettingsList("trusted_modules", "TrustedModules", t.Entries, false)
}
