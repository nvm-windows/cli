package firewall

import (
	"common/acl"
	"common/modulefirewall"
	"common/preferences"
	"common/registry"
	"common/settings"
	"common/system"
	"fmt"
	"nvm/log"
	"strings"
)

// Event codes (NVM44xx firewall range).
const (
	CodePolicyMutate      = 4401
	CodeModuleBlocked     = 4402
	CodeRemoteFailed      = 4403
	CodeElevationRequired = 4404
	CodeInvalidRule       = 4405
)

type Root struct {
	Allow AllowRoot `cmd:"allow" help:"Allow versions or modules through the NVM firewall (certified)."`
	Deny  DenyRoot  `cmd:"deny" help:"Deny versions or modules through the NVM firewall (certified)."`
	Trust TrustRoot `cmd:"trust" help:"Manage TrustedModules for self-updating global CLIs."`
}

type AllowRoot struct {
	Version AllowVersion `cmd:"version" help:"Append entries to VersionAllowList (HKLM; requires elevation)."`
	Module  AllowModule  `cmd:"module" help:"Append entries to ApprovedModules / ApprovedGlobalModules (HKLM; requires elevation)."`
}

type DenyRoot struct {
	Version DenyVersion `cmd:"version" help:"Append NOT entries to VersionAllowList (HKLM; requires elevation)."`
	Module  DenyModule  `cmd:"module" help:"Append NOT entries to ApprovedModules / ApprovedGlobalModules (HKLM; requires elevation)."`
}

type TrustRoot struct {
	Module TrustModule `cmd:"module" help:"Manage TrustedModules (self-update auto-reshim allow list)."`
}

type AllowVersion struct {
	Entries []string `arg:"" name:"entry" help:"Version rules (semver, 20.x, aliases, ALL)."`
}

type DenyVersion struct {
	Entries []string `arg:"" name:"entry" help:"Version rules to deny (stored as NOT <entry> on VersionAllowList)."`
}

type AllowModule struct {
	Entries []string `arg:"" name:"entry" help:"Module patterns (name, @org/*, name@1.*, name@>=1.0.0)."`
	Global  bool     `name:"global" help:"Write ApprovedGlobalModules instead of ApprovedModules."`
}

type DenyModule struct {
	Entries []string `arg:"" name:"entry" help:"Module patterns to deny (stored as NOT <entry>)."`
	Global  bool     `name:"global" help:"Write ApprovedGlobalModules instead of ApprovedModules."`
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

func requireCertified() error {
	if acl.Implementation() != "policy" {
		return fmt.Errorf("module/version firewall allow|deny requires a Certified Build")
	}
	return nil
}

func policyKey(name string) string {
	root := strings.TrimSpace(preferences.MACHINE_POLICY_ROOT)
	if root == "" {
		root = "HKLM/SOFTWARE/Policies/Author Software/nvm"
	}
	return root + "/" + name
}

func readStringList(regPath string) ([]string, error) {
	value, exists, err := registry.Get(regPath)
	if err != nil {
		return nil, err
	}
	if !exists || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case []string:
		return append([]string{}, v...), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	default:
		return nil, nil
	}
}

func appendPolicyMulti(regName string, add []string, negate bool) error {
	if err := requireMachine(); err != nil {
		return err
	}
	path := policyKey(regName)
	list, err := readStringList(path)
	if err != nil {
		return err
	}
	for _, e := range add {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if negate && !strings.HasPrefix(strings.ToLower(e), "not ") && !strings.HasPrefix(e, "!") {
			e = "NOT " + e
		}
		list = append(list, e)
	}
	if err := registry.Put(list, path); err != nil {
		log.ErrorStructured("firewall.policy_mutate_failed", map[string]any{
			"key":   regName,
			"error": err.Error(),
		}, CodePolicyMutate)
		return err
	}
	log.Logf("firewall: updated %s (+%d entries)", regName, len(add))
	log.LogStructured("firewall.policy_mutated", map[string]any{
		"key":    regName,
		"added":  add,
		"negate": negate,
		"action": "append",
	}, CodePolicyMutate)
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

func (a *AllowVersion) Run() error {
	if err := requireCertified(); err != nil {
		return err
	}
	return appendPolicyMulti("VersionAllowList", a.Entries, false)
}

func (d *DenyVersion) Run() error {
	if err := requireCertified(); err != nil {
		return err
	}
	return appendPolicyMulti("VersionAllowList", d.Entries, true)
}

func (a *AllowModule) Run() error {
	if err := requireCertified(); err != nil {
		return err
	}
	key, label := "approved_modules", "ApprovedModules"
	if a.Global {
		key, label = "approved_global_modules", "ApprovedGlobalModules"
	}
	return appendSettingsList(key, label, a.Entries, false)
}

func (d *DenyModule) Run() error {
	if err := requireCertified(); err != nil {
		return err
	}
	key, label := "approved_modules", "ApprovedModules"
	if d.Global {
		key, label = "approved_global_modules", "ApprovedGlobalModules"
	}
	return appendSettingsList(key, label, d.Entries, true)
}

func (t *TrustModule) Run() error {
	return appendSettingsList("trusted_modules", "TrustedModules", t.Entries, false)
}
