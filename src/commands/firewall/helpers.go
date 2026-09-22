package firewall

import (
	"bufio"
	"common/modulefirewall"
	"common/notify"
	"common/settings"
	"common/system"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"nvm/log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func enrichSIEM(m map[string]any) {
	m["user"] = log.Actor()
	m["sid"] = log.ActorSid()
	m["hostname"] = log.Hostname()
	m["correlation_id"] = log.NewCorrelationID()
}

func packageName(pkg modulefirewall.PackageSpec) string {
	if pkg.Raw != "" {
		return pkg.Raw
	}
	return pkg.Name
}

func packageNames(pkgs []modulefirewall.PackageSpec) []string {
	out := make([]string, 0, len(pkgs))
	for _, pkg := range pkgs {
		out = append(out, packageName(pkg))
	}
	return out
}

// logRemoteTrustEvent writes operational plaintext (community) and structured
// payload (certified / licensed). Community LogStructured falls back to plaintext.
func logRemoteTrustEvent(event string, code int, plain string, payload map[string]any) {
	enrichSIEM(payload)
	if plain != "" {
		log.Log(plain, code)
	}
	switch code {
	case CodeRemoteFailed, CodeModuleBlocked:
		log.ErrorStructured(event, payload, code)
	default:
		log.LogStructured(event, payload, code)
	}
}

// CheckRemoteTrust is an internal helper for proxy.exe HTTPS TrustedModules evaluation.
// Local list is checked first; HTTP runs only for modules not trusted locally.
// Exit: 0 trusted, 1 remote 403 / local untrusted, 2 request failed (NVM4402).
type CheckRemoteTrust struct {
	Modules []string `arg:"" name:"module" help:"Package tokens to evaluate against TrustedModules."`
}

func (c *CheckRemoteTrust) Run() error {
	cfg := settings.Global()
	pkgs := make([]modulefirewall.PackageSpec, 0, len(c.Modules))
	for _, m := range c.Modules {
		pkg, err := modulefirewall.ParsePackageToken(m)
		if err != nil {
			log.ErrorStructured("firewall.invalid_rule", map[string]any{
				"entry": m,
				"error": err.Error(),
			}, CodeInvalidRule)
			return fmt.Errorf("invalid module token %q (NVM%d): %w", m, CodeInvalidRule, err)
		}
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		return nil
	}

	res := modulefirewall.EvaluateTrustedModules(pkgs, cfg.TrustedModules, modulefirewall.RemoteTLSOptions{
		TimeoutSec: cfg.FirewallHTTPTimeoutSeconds,
	})
	if res.Trusted {
		if res.RemoteQueried {
			logRemoteTrustEvent("firewall.remote_trust_allowed", CodeRemoteAllowed, "NVM Firewall: remote policy allowed", map[string]any{
				"count":  len(pkgs),
				"remote": true,
				"status": res.Remote.Status,
			})
		}
		return nil
	}

	if res.Message != "" {
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s\n", res.Message)
	}
	if res.RemoteQueried && modulefirewall.RemoteTrustUnavailable(res.Remote) {
		logRemoteTrustEvent("firewall.remote_trust_unavailable", CodeRemoteFailed, "NVM Firewall: "+res.Message, map[string]any{
			"error":   res.Message,
			"status":  res.Remote.Status,
			"detail":  res.Remote.ErrorMsg,
			"modules": packageNames(res.Untrusted),
		})
		os.Exit(2)
		return nil
	}
	if res.RemoteQueried && res.Remote.Status == 403 {
		logRemoteTrustEvent("firewall.remote_blocked", CodeModuleBlocked, "NVM Firewall: blocked by remote policy", map[string]any{
			"status":  403,
			"modules": packageNames(res.Untrusted),
			"error":   res.Message,
		})
		os.Exit(1)
		return nil
	}
	for _, u := range res.Untrusted {
		fmt.Fprintf(os.Stderr, "  untrusted: %s\n", packageName(u))
	}
	os.Exit(1)
	return nil
}

// PromptTrust asks whether to trust a module after a self-update.
// Foreground console: interactive y/N only (no toast).
// Background: native toast with Trust / Cancel (protocol actions); wait for answer.
type PromptTrust struct {
	Module string `arg:"" name:"module" help:"Module / command name that changed."`
}

func (p *PromptTrust) Run() error {
	name := strings.TrimSpace(p.Module)
	if name == "" {
		return fmt.Errorf("module name required")
	}

	if system.IsAppInForeground() {
		return promptTrustConsole(name)
	}
	return promptTrustToast(name)
}

// NotifyChanged fires a quiet toast when UntrustedModuleHandlerAction=allow
// auto-reshims after a module entrypoint change (no Trust/Cancel actions).
type NotifyChanged struct {
	Module string `arg:"" name:"module" help:"Module / command name that changed."`
}

func (n *NotifyChanged) Run() error {
	name := strings.TrimSpace(n.Module)
	if name == "" {
		return fmt.Errorf("module name required")
	}
	msg := fmt.Sprintf("A change to module '%s' was automatically trusted.", name)
	_ = notify.Send(settings.AppId, "NVM Firewall", msg)
	// NVM4406 audit emitted by proxy at decision site.
	return nil
}

// logUntrustedModuleChanged emits NVM4406 for an untrusted module change
// (plain text + structured). Used when CLI is the decision site (tests / future).
// outcome: deny|allow|prompt_accepted|prompt_declined.
func logUntrustedModuleChanged(module, outcome, handler, via string) {
	plain := fmt.Sprintf(
		"NVM%d Untrusted module '%s' changed (outcome=%s, handler=%s, via=%s)",
		CodeUntrustedModuleChanged, module, outcome, handler, via,
	)
	log.Log(plain, CodeUntrustedModuleChanged)
	log.LogStructured("firewall.untrusted_module_changed", map[string]any{
		"module":  module,
		"outcome": outcome,
		"handler": handler,
		"via":     via,
	}, CodeUntrustedModuleChanged)
}

func promptTrustConsole(name string) error {
	msg := fmt.Sprintf("Untrusted module '%s' changed after running. Do you trust this module?", name)
	fmt.Fprintf(os.Stderr, "NVM Firewall: %s [y/N]: ", msg)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s is not trusted\n", name)
		os.Exit(1)
		return nil
	}
	line = strings.TrimSpace(line)
	ok := len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
	if !ok {
		fmt.Fprintf(os.Stderr, "%s is not trusted\n", name)
		os.Exit(1)
		return nil
	}
	if err := appendTrusted([]string{name}, false); err != nil {
		return err
	}
	return nil
}

func promptTrustToast(name string) error {
	token, err := newTrustToken()
	if err != nil {
		return err
	}
	rspPath := trustResponsePath(token)
	if err := os.WriteFile(rspPath, []byte("pending\n"), 0o600); err != nil {
		return err
	}
	defer os.Remove(rspPath)

	msg := fmt.Sprintf("Untrusted module '%s' changed after running. Do you trust this module?", name)
	_ = notify.Send(
		settings.AppId,
		"NVM Firewall",
		msg,
		notify.Action{
			Label: "Trust",
			URL:   fmt.Sprintf("nvm://firewall?action=trust&module=%s&token=%s", url.QueryEscape(name), token),
		},
		notify.Action{
			Label: "Cancel",
			URL:   fmt.Sprintf("nvm://firewall?action=cancel&module=%s&token=%s", url.QueryEscape(name), token),
		},
	)

	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		raw, readErr := os.ReadFile(rspPath)
		if readErr == nil {
			switch strings.TrimSpace(string(raw)) {
			case "accept":
				return nil
			case "decline":
				fmt.Fprintf(os.Stderr, "%s is not trusted\n", name)
				os.Exit(1)
				return nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}

	fmt.Fprintf(os.Stderr, "Trust prompt timed out for %s; reshim skipped.\n", name)
	os.Exit(1)
	return nil
}

// HandleProtocolURI handles nvm://firewall?action=trust|cancel&module=&token= from toast buttons.
func HandleProtocolURI(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid nvm protocol URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "nvm") {
		return fmt.Errorf("unsupported protocol %q", u.Scheme)
	}
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if host == "" {
		host = strings.ToLower(strings.Trim(u.Path, "/"))
	}
	if host != "firewall" {
		return fmt.Errorf("unsupported nvm:// host %q", host)
	}

	q := u.Query()
	action := strings.ToLower(strings.TrimSpace(q.Get("action")))
	module := strings.TrimSpace(q.Get("module"))
	token := strings.TrimSpace(q.Get("token"))
	if !validTrustToken(token) {
		return fmt.Errorf("invalid trust token")
	}
	rspPath := trustResponsePath(token)

	switch action {
	case "trust":
		if module == "" {
			return fmt.Errorf("module required")
		}
		if err := appendTrusted([]string{module}, false); err != nil {
			return err
		}
		_ = os.WriteFile(rspPath, []byte("accept\n"), 0o600)
		// NVM4406 emitted by prompt-trust waiter when it reads accept.
		return nil
	case "cancel", "decline":
		_ = os.WriteFile(rspPath, []byte("decline\n"), 0o600)
		// NVM4406 emitted by prompt-trust waiter when it reads decline.
		return nil
	default:
		return fmt.Errorf("unknown firewall action %q", action)
	}
}

func newTrustToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func validTrustToken(token string) bool {
	if len(token) < 16 || len(token) > 64 {
		return false
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

func trustResponsePath(token string) string {
	return filepath.Join(os.TempDir(), "nvm-fw-trust-"+token+".rsp")
}
