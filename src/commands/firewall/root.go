package firewall

import (
	"common/modulefirewall"
	"common/preferences"
	"common/registry"
	"common/settings"
	"common/system"
	"encoding/json"
	"fmt"
	"nvm/constant"
	"nvm/log"
	"os"
	"strings"
)

// Event codes (NVM44xx firewall range).
const (
	CodeRemoteUnauthorized     = 4401 // unused in community; reserved to match certified
	CodeRemoteFailed           = 4402
	CodeModuleBlocked          = 4403
	CodeElevationRequired      = 4404
	CodeInvalidRule            = 4405
	CodeUntrustedModuleChanged = 4406
	CodeRemoteAllowed          = 4407
	CodeModuleAllowed          = 4408 // unused in community; reserved to match certified
	CodeRemoteUnreachable      = 4409 // unused in community; reserved to match certified
	CodePolicyMutate           = 4410
)

const trustedModulesCfg = "trusted_modules"
const trustedModulesReg = "TrustedModules"

type Root struct {
	Trust           TrustRoot       `cmd:"trust" help:"Trust global module installations."`
	Distrust        DistrustRoot    `cmd:"distrust" help:"Distrust global module installations."`
	PromptTrust     PromptTrust     `cmd:"prompt-trust" hidden:"true" help:"Internal: dual-channel trust prompt for proxy."`
	NotifyChanged   NotifyChanged   `cmd:"notify-changed" hidden:"true" help:"Internal: quiet toast when allow-mode module changes."`
	CheckRemoteTrust CheckRemoteTrust `cmd:"check-remote-trust" hidden:"true" help:"Internal: HTTPS TrustedModules evaluation for proxy."`
}

type TrustRoot struct {
	Module TrustModule `cmd:"module" help:"Manage TrustedModules (self-update auto-reshim allow list)."`
}

type DistrustRoot struct {
	Module DistrustModuleRoot `cmd:"module" help:"Remove modules from TrustedModules / list current TrustedModules."`
}

type DistrustModuleRoot struct {
	List   DistrustModuleList `cmd:"list" aliases:"ls" help:"List TrustedModules (same registry as trust module list)."`
	Remove DistrustModule     `cmd:"" default:"withargs" help:"Remove modules from TrustedModules."`
}

type DistrustModuleList struct {
	constant.FlagJSON
}

type DistrustModule struct {
	Machine bool     `name:"machine" help:"Distrust modules for the entire machine."`
	Entries []string `arg:"" optional:"" name:"entry" help:"Module patterns to remove from TrustedModules."`
}

type TrustModule struct {
	List TrustModuleList `cmd:"list" aliases:"ls" help:"List trusted modules."`
	Add  TrustModuleAdd  `cmd:"add" default:"withargs" help:"Add modules to TrustedModules (self-update auto-reshim allow list)."`
}

type TrustModuleAdd struct {
	Machine bool     `name:"machine" help:"Trust modules for the entire machine."`
	Entries []string `arg:"" name:"entry" help:"Module patterns to trust for auto-reshim."`
}

type TrustModuleList struct {
	constant.FlagJSON
}

func requireMachine() error {
	if err := system.RequireAdministrator(); err != nil {
		payload := map[string]any{"error": err.Error()}
		enrichSIEM(payload)
		log.ErrorStructured("firewall.elevation_required", payload, CodeElevationRequired)
		return fmt.Errorf("firewall policy changes require an elevated administrator prompt (NVM%d): %w", CodeElevationRequired, err)
	}
	return nil
}

func normalizeEntry(e string) string {
	return strings.TrimSpace(e)
}

func entryKey(e string) string {
	return strings.ToLower(normalizeEntry(e))
}

func stripNegation(e string) string {
	e = normalizeEntry(e)
	lower := strings.ToLower(e)
	if strings.HasPrefix(lower, "not ") || strings.HasPrefix(lower, "not\t") {
		return strings.TrimSpace(e[3:])
	}
	if strings.HasPrefix(e, "!") {
		return strings.TrimSpace(e[1:])
	}
	return e
}

func dedupeList(list []string) []string {
	seen := make(map[string]struct{}, len(list))
	out := make([]string, 0, len(list))
	for _, e := range list {
		e = normalizeEntry(e)
		if e == "" {
			continue
		}
		k := entryKey(e)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, e)
	}
	return out
}

func valueToStringList(cur interface{}) []string {
	switch v := cur.(type) {
	case []string:
		return append([]string{}, v...)
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return nil
		}
		parts := strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r'
		})
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = normalizeEntry(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	default:
		return nil
	}
}

func readTrustedList(machine bool) []string {
	if machine {
		list, _ := readTrustedAtRoot(preferences.MACHINE_PREFERENCE_ROOT)
		return list
	}
	list, _ := readTrustedAtRoot(preferences.USER_PREFERENCE_ROOT)
	if list != nil {
		return list
	}
	// Fallback when USER_PREFERENCE_ROOT unset.
	root := strings.TrimRight(strings.TrimSpace(preferences.ROOT), "/")
	if root == "" {
		cur, _ := settings.Get(trustedModulesCfg)
		return valueToStringList(cur)
	}
	list, _ = readTrustedAtRoot(root)
	return list
}

// readEffectiveTrustedList returns the active trust list: HKLM overrides HKCU when present.
func readEffectiveTrustedList() []string {
	if list, ok := readTrustedAtRoot(preferences.MACHINE_PREFERENCE_ROOT); ok {
		return list
	}
	return readTrustedList(false)
}

func readTrustedAtRoot(root string) ([]string, bool) {
	root = strings.TrimRight(strings.TrimSpace(root), "/")
	if root == "" {
		return nil, false
	}
	value, exists, err := registry.Get(root + "/" + trustedModulesReg)
	if err != nil || !exists || value == nil {
		return nil, false
	}
	return valueToStringList(value), true
}

func writeTrustedList(list []string, machine bool) error {
	list = dedupeList(list)
	if machine {
		if err := requireMachine(); err != nil {
			return err
		}
		if len(list) == 0 {
			if err := settings.DelMachine(trustedModulesCfg); err != nil {
				payload := map[string]any{
					"key":   trustedModulesCfg,
					"error": err.Error(),
				}
				enrichSIEM(payload)
				log.ErrorStructured("firewall.policy_mutate_failed", payload, CodePolicyMutate)
				return err
			}
		} else if err := settings.PutMachine(trustedModulesCfg, strings.Join(list, ",")); err != nil {
			payload := map[string]any{
				"key":   trustedModulesCfg,
				"error": err.Error(),
			}
			enrichSIEM(payload)
			log.ErrorStructured("firewall.policy_mutate_failed", payload, CodePolicyMutate)
			return err
		}
	} else {
		if len(list) == 0 {
			if err := settings.Del(trustedModulesCfg); err != nil {
				payload := map[string]any{
					"key":   trustedModulesCfg,
					"error": err.Error(),
				}
				enrichSIEM(payload)
				log.ErrorStructured("firewall.policy_mutate_failed", payload, CodePolicyMutate)
				return err
			}
		} else if err := settings.Put(trustedModulesCfg, strings.Join(list, ",")); err != nil {
			payload := map[string]any{
				"key":   trustedModulesCfg,
				"error": err.Error(),
			}
			enrichSIEM(payload)
			log.ErrorStructured("firewall.policy_mutate_failed", payload, CodePolicyMutate)
			return err
		}
	}
	return nil
}

func appendTrusted(entries []string, machine bool) error {
	list := readTrustedList(machine)
	added := 0
	for _, e := range entries {
		e = normalizeEntry(e)
		if e == "" {
			continue
		}
		if err := modulefirewall.ValidateRuleEntry(e); err != nil {
			payload := map[string]any{
				"key":   trustedModulesCfg,
				"entry": e,
				"error": err.Error(),
			}
			enrichSIEM(payload)
			log.ErrorStructured("firewall.invalid_rule", payload, CodeInvalidRule)
			return fmt.Errorf("invalid firewall entry %q (NVM%d): %w", e, CodeInvalidRule, err)
		}
		list = append(list, e)
		added++
	}
	list = dedupeList(list)
	if err := writeTrustedList(list, machine); err != nil {
		return err
	}
	for _, e := range entries {
		e = normalizeEntry(e)
		if e == "" {
			continue
		}
		fmt.Printf("%s is now trusted\n", e)
	}
	scope := "user"
	if machine {
		scope = "machine"
	}
	log.Logf("firewall: updated %s (+%d entries, %s)", trustedModulesReg, added, scope)
	payload := map[string]any{
		"key":     trustedModulesCfg,
		"added":   entries,
		"action":  "append",
		"machine": machine,
	}
	enrichSIEM(payload)
	log.LogStructured("firewall.policy_mutated", payload, CodePolicyMutate)
	return nil
}

func entryMatchesRemove(stored, want string) bool {
	stored = normalizeEntry(stored)
	want = normalizeEntry(want)
	if stored == "" || want == "" {
		return false
	}
	if entryKey(stored) == entryKey(want) {
		return true
	}
	return entryKey(stripNegation(stored)) == entryKey(stripNegation(want))
}

func removeTrusted(entries []string, machine bool) error {
	list := readTrustedList(machine)
	removeSet := make([]string, 0, len(entries))
	for _, e := range entries {
		e = normalizeEntry(e)
		if e == "" {
			continue
		}
		// Prefer ValidateRuleEntry; bare names still proceed even if loose.
		if err := modulefirewall.ValidateRuleEntry(e); err != nil {
			bare := stripNegation(e)
			if bare == "" || strings.ContainsAny(bare, " \t") {
				payload := map[string]any{
					"key":   trustedModulesCfg,
					"entry": e,
					"error": err.Error(),
				}
				enrichSIEM(payload)
				log.ErrorStructured("firewall.invalid_rule", payload, CodeInvalidRule)
				return fmt.Errorf("invalid firewall entry %q (NVM%d): %w", e, CodeInvalidRule, err)
			}
			// simple name — still attempt removal
		}
		removeSet = append(removeSet, e)
	}

	kept := make([]string, 0, len(list))
	removed := 0
	for _, stored := range list {
		drop := false
		for _, want := range removeSet {
			if entryMatchesRemove(stored, want) {
				drop = true
				break
			}
		}
		if drop {
			removed++
			continue
		}
		kept = append(kept, stored)
	}
	kept = dedupeList(kept)
	if err := writeTrustedList(kept, machine); err != nil {
		return err
	}
	scope := "user"
	if machine {
		scope = "machine"
	}
	log.Logf("firewall: updated %s (-%d entries, %s)", trustedModulesReg, removed, scope)
	payload := map[string]any{
		"key":     trustedModulesCfg,
		"removed": entries,
		"action":  "remove",
		"machine": machine,
	}
	enrichSIEM(payload)
	log.LogStructured("firewall.policy_mutated", payload, CodePolicyMutate)
	return nil
}

func (t *TrustModuleAdd) Run() error {
	if len(t.Entries) == 0 {
		return fmt.Errorf("entry required (or use: nvm firewall trust module list)")
	}
	return appendTrusted(t.Entries, t.Machine)
}

func (t *TrustModuleList) Run() error {
	return printTrustedModulesList(t.JSON)
}

func (d *DistrustModuleList) Run() error {
	return printTrustedModulesList(d.JSON)
}

func printTrustedModulesList(asJSON bool) error {
	list := readEffectiveTrustedList()
	if asJSON {
		if list == nil {
			list = []string{}
		}
		out, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal TrustedModules to JSON: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}
	if len(list) == 0 {
		fmt.Println("no trusted modules")
		return nil
	}
	modulefirewall.FormatHumanList(os.Stdout, list)
	return nil
}

func (u *DistrustModule) Run() error {
	if len(u.Entries) == 0 {
		return fmt.Errorf("entry required (or use: nvm firewall distrust module list)")
	}
	return removeTrusted(u.Entries, u.Machine)
}
