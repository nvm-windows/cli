package firewall

import (
	"bufio"
	"common/mirrorauth"
	"common/modulefirewall"
	"common/notify"
	"common/settings"
	"common/system"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	neturl "net/url"
	"nvm/log"
	"os"
	"path/filepath"
	"strings"
	"time"

	license "common/license"
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

// logRemoteTrustEvent writes operational plaintext and structured payload.
// Certified LogStructured/ErrorStructured emit ETW structured events.
func logRemoteTrustEvent(event string, code int, plain string, payload map[string]any) {
	enrichSIEM(payload)
	if plain != "" {
		log.Log(plain, code)
	}
	switch code {
	case CodeRemoteFailed, CodeRemoteUnreachable, CodeRemoteUnauthorized, CodeModuleBlocked:
		log.ErrorStructured(event, payload, code)
	default:
		log.LogStructured(event, payload, code)
	}
}

func firewallUA() string {
	build := "certified"
	if license.IsCommunityBuild() {
		build = "community"
	}
	return modulefirewall.FirewallUserAgent(settings.ProductVersion, build)
}

func buildFirewallRemoteOpts(endpoint, shim string, art modulefirewall.ManifestArtifact, tls modulefirewall.RemoteTLSOptions) modulefirewall.RemoteRequestOptions {
	cwd, _ := os.Getwd()
	cfg := settings.Global()
	ctx := modulefirewall.BuildRequestContextWithRoot(cwd, shim, cfg.ActiveVersion, settings.Expand(cfg.Root))
	aud := mirrorauth.FirewallAudienceHost(endpoint)
	token, err := mirrorauth.MintFirewallJWT(aud, ctx)
	if err != nil {
		log.Logf("firewall jwt mint failed: %v", err)
	}

	opts := modulefirewall.RemoteRequestOptions{
		RemoteTLSOptions: tls,
		UserAgent:        firewallUA(),
		SpinnerAfter:     200 * time.Millisecond,
	}
	if strings.TrimSpace(token) != "" {
		opts.Authorization = token
	}
	if art.Source == "package.json" && len(art.Body) > 0 {
		opts.Body = art.Body
		opts.ContentType = art.ContentType
		if art.PackageShasum != "" {
			opts.ExtraHeaders = map[string]string{"x-nvm-package-shasum": art.PackageShasum}
		}
	} else if art.Source == "lock" && len(art.Body) > 0 {
		opts.Body = art.Body
		opts.ContentType = art.ContentType
	}
	return opts
}

// CheckRemote is an internal helper for proxy.exe HTTPS module-firewall evaluation.
// Exit semantics: 0 allow, 1 deny/block, 2 transport/config error (NVM4402 / NVM4409).
type CheckRemote struct {
	Global  bool     `flag:"global" help:"Evaluate ApprovedGlobalModules instead of ApprovedModules."`
	Shim    string   `flag:"shim" help:"Proxied entrypoint name (npm, npx, pnpm, yarn, vlt)."`
	Cwd     string   `flag:"cwd" help:"Working directory for manifest resolution (default: process cwd)."`
	OmitDev bool     `flag:"omit-dev" help:"Exclude package.json devDependencies (production-like install)."`
	Modules []string `arg:"" optional:"" name:"module" help:"Package tokens to POST (name or name@version). Empty = expand from package.json/lock."`
}

func (c *CheckRemote) Run() error {
	cfg := settings.Global()
	list := cfg.ApprovedModules
	keyName := "ApprovedModules"
	if c.Global {
		list = cfg.ApprovedGlobalModules
		keyName = "ApprovedGlobalModules"
	}
	url, ok := modulefirewall.ExtractHTTPSURL(list)
	if !ok {
		log.Logf("firewall check-remote: %s has no HTTPS URL; allowing (local eval expected in proxy)", keyName)
		return nil
	}

	shim := strings.TrimSpace(c.Shim)
	if shim == "" {
		shim = "npm"
	}
	cwd := strings.TrimSpace(c.Cwd)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	installArgs := []string{"install"}
	if c.OmitDev {
		installArgs = append(installArgs, "--production")
	}

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

	var art modulefirewall.ManifestArtifact
	if len(pkgs) == 0 {
		// Bare install: expand from lock (default) or package.json.
		var err error
		art, err = modulefirewall.BuildManifestArtifact(modulefirewall.CollectOptions{
			Cwd:          cwd,
			Command:      shim,
			Args:         installArgs,
			SkipLockfile: cfg.FirewallSkipLockfile,
		})
		if err != nil {
			return err
		}
		pkgs = art.Modules
		if len(pkgs) == 0 && art.Source != "package.json" {
			return nil
		}
	} else {
		// Explicit CLI packages win — newline module list body (no lock replace).
		art = modulefirewall.ManifestArtifact{Source: "cli", Modules: pkgs}
	}

	tlsOpts := modulefirewall.RemoteTLSOptions{
		TimeoutSec:         cfg.FirewallHTTPTimeoutSeconds,
		AllowedOrgs:        cfg.TrustedFirewallSigners,
		AllowedThumbprints: cfg.TrustedFirewallThumbprint,
	}
	reqOpts := buildFirewallRemoteOpts(url, shim, art, tlsOpts)

	logRemoteTrustEvent("firewall.remote_request", CodeRemoteAllowed, "", map[string]any{
		"url_host": mirrorauth.FirewallAudienceHost(url),
		"shim":     shim,
		"source":   art.Source,
		"count":    len(pkgs),
	})

	var res modulefirewall.RemoteResult
	if art.Source == "package.json" || art.Source == "lock" {
		res = modulefirewall.EvaluateRemoteRequest(url, pkgs, reqOpts)
	} else {
		res = modulefirewall.EvaluateRemoteRequest(url, pkgs, reqOpts)
	}

	if modulefirewall.RemoteTrustUnavailable(res) {
		code := CodeRemoteFailed
		if res.Unreachable {
			code = CodeRemoteUnreachable
		}
		msg := modulefirewall.FormatRemoteUserMessage(res)
		logRemoteTrustEvent("firewall.remote_failed", code, "NVM Firewall: "+msg, map[string]any{
			"url":         url,
			"error":       res.ErrorMsg,
			"status":      res.Status,
			"unreachable": res.Unreachable,
			"shim":        shim,
		})
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s (NVM%d)\n", msg, code)
		os.Exit(2)
	}
	if !res.Allowed {
		code := CodeModuleBlocked
		event := "firewall.remote_blocked"
		if res.Status == 401 {
			code = CodeRemoteUnauthorized
			event = "firewall.remote_unauthorized"
		}
		msg := modulefirewall.FormatRemoteUserMessage(res)
		logRemoteTrustEvent(event, code, "NVM Firewall: "+msg, map[string]any{
			"url":    url,
			"status": res.Status,
			"blocks": res.Blocks,
			"error":  res.ErrorMsg,
			"shim":   shim,
		})
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s (NVM%d)\n", msg, code)
		if len(res.Blocks) > 0 {
			lines := make([]string, 0, len(res.Blocks))
			for _, b := range res.Blocks {
				line := b.Name
				if b.Reason != "" {
					line = b.Name + "\t" + b.Reason
				}
				lines = append(lines, line)
			}
			modulefirewall.FormatHumanList(os.Stderr, lines)
		}
		os.Exit(1)
	}
	logRemoteTrustEvent("firewall.remote_allowed", CodeRemoteAllowed, "NVM Firewall: remote policy allowed", map[string]any{
		"url":    url,
		"status": res.Status,
		"count":  len(pkgs),
		"shim":   shim,
		"source": art.Source,
	})
	return nil
}

// CheckRemoteTrust is an internal helper for proxy.exe HTTPS TrustedModules evaluation.
// Local list is checked first; HTTP runs only for modules not trusted locally.
// Exit: 0 trusted, 1 remote 403/401 / local untrusted, 2 request failed (NVM4402 / NVM4409).
type CheckRemoteTrust struct {
	Shim    string   `flag:"shim" help:"Proxied entrypoint that triggered trust evaluation."`
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

	shim := strings.TrimSpace(c.Shim)
	if shim == "" {
		shim = pkgs[0].Name
	}

	tlsOpts := modulefirewall.RemoteTLSOptions{
		TimeoutSec:         cfg.FirewallHTTPTimeoutSeconds,
		AllowedOrgs:        cfg.TrustedFirewallSigners,
		AllowedThumbprints: cfg.TrustedFirewallThumbprint,
	}
	endpoint, _ := modulefirewall.ExtractHTTPSURL(cfg.TrustedModules)
	art := modulefirewall.ManifestArtifact{Source: "cli", Modules: pkgs}
	reqOpts := buildFirewallRemoteOpts(endpoint, shim, art, tlsOpts)

	res := modulefirewall.EvaluateTrustedModulesRequest(pkgs, cfg.TrustedModules, reqOpts)
	if res.Trusted {
		if res.RemoteQueried {
			logRemoteTrustEvent("firewall.remote_trust_allowed", CodeRemoteAllowed, "NVM Firewall: remote policy allowed", map[string]any{
				"count":  len(pkgs),
				"remote": true,
				"status": res.Remote.Status,
				"shim":   shim,
			})
		}
		return nil
	}

	if res.RemoteQueried && modulefirewall.RemoteTrustUnavailable(res.Remote) {
		code := CodeRemoteFailed
		if res.Remote.Unreachable {
			code = CodeRemoteUnreachable
		}
		msg := res.Message
		if strings.TrimSpace(msg) == "" {
			msg = modulefirewall.FormatRemoteUserMessage(res.Remote)
		}
		logRemoteTrustEvent("firewall.remote_trust_unavailable", code, "NVM Firewall: "+msg, map[string]any{
			"error":       msg,
			"status":      res.Remote.Status,
			"detail":      res.Remote.ErrorMsg,
			"unreachable": res.Remote.Unreachable,
			"modules":     packageNames(res.Untrusted),
			"shim":        shim,
		})
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s (NVM%d)\n", msg, code)
		os.Exit(2)
		return nil
	}
	if res.Message != "" {
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s\n", res.Message)
	}
	if res.RemoteQueried && res.Remote.Status == 401 {
		msg := modulefirewall.FormatRemoteUserMessage(res.Remote)
		logRemoteTrustEvent("firewall.remote_unauthorized", CodeRemoteUnauthorized, "NVM Firewall: "+msg, map[string]any{
			"status":  401,
			"modules": packageNames(res.Untrusted),
			"error":   msg,
			"shim":    shim,
		})
		fmt.Fprintf(os.Stderr, "NVM Firewall: %s (NVM%d)\n", msg, CodeRemoteUnauthorized)
		os.Exit(1)
		return nil
	}
	if res.RemoteQueried && res.Remote.Status == 403 {
		logRemoteTrustEvent("firewall.remote_blocked", CodeModuleBlocked, "NVM Firewall: blocked by remote policy", map[string]any{
			"status":  403,
			"modules": packageNames(res.Untrusted),
			"error":   res.Message,
			"shim":    shim,
		})
		os.Exit(1)
		return nil
	}
	lines := make([]string, 0, len(res.Untrusted))
	for _, u := range res.Untrusted {
		lines = append(lines, "  untrusted: "+packageName(u))
	}
	modulefirewall.FormatHumanList(os.Stderr, lines)
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
			URL:   fmt.Sprintf("nvm://firewall?action=trust&module=%s&token=%s", neturl.QueryEscape(name), token),
		},
		notify.Action{
			Label: "Cancel",
			URL:   fmt.Sprintf("nvm://firewall?action=cancel&module=%s&token=%s", neturl.QueryEscape(name), token),
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
	u, err := neturl.Parse(strings.TrimSpace(raw))
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
		return nil
	case "cancel", "decline":
		_ = os.WriteFile(rspPath, []byte("decline\n"), 0o600)
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
