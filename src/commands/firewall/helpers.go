package firewall

import (
	"bufio"
	"common/modulefirewall"
	"common/notify"
	"common/settings"
	"fmt"
	"nvm/log"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CheckRemote is an internal helper for proxy.exe HTTPS module-firewall evaluation.
// Exit semantics: 0 allow, 1 deny/block, 2 transport/config error (NVM4403).
type CheckRemote struct {
	Global  bool     `flag:"global" help:"Evaluate ApprovedGlobalModules instead of ApprovedModules."`
	Modules []string `arg:"" name:"module" help:"Package tokens to POST (name or name@version)."`
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
		// Local rules belong in the shim; helper is HTTPS-only.
		log.Logf("firewall check-remote: %s has no HTTPS URL; allowing (local eval expected in proxy)", keyName)
		return nil
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
	if len(pkgs) == 0 {
		return nil
	}

	res := modulefirewall.EvaluateRemote(url, pkgs, modulefirewall.RemoteTLSOptions{
		TimeoutSec:         cfg.FirewallHTTPTimeoutSeconds,
		AllowedOrgs:        cfg.TrustedFirewallSigners,
		AllowedThumbprints: cfg.TrustedFirewallThumbprint,
	})
	if res.ErrorMsg != "" && res.Status == 0 {
		log.ErrorStructured("firewall.remote_failed", map[string]any{
			"url":   url,
			"error": res.ErrorMsg,
		}, CodeRemoteFailed)
		fmt.Fprintf(os.Stderr, "NVM%d %s\n", CodeRemoteFailed, res.ErrorMsg)
		os.Exit(2)
	}
	if !res.Allowed {
		log.ErrorStructured("firewall.remote_blocked", map[string]any{
			"url":    url,
			"status": res.Status,
			"blocks": res.Blocks,
			"error":  res.ErrorMsg,
		}, CodeModuleBlocked)
		fmt.Fprintf(os.Stderr, "NVM%d Module firewall remote policy blocked install.\n", CodeModuleBlocked)
		for _, b := range res.Blocks {
			fmt.Fprintf(os.Stderr, "  blocked: %s\t%s\t%s\n", b.Name, b.Date, b.Reason)
		}
		if len(res.Blocks) == 0 && res.ErrorMsg != "" {
			fmt.Fprintf(os.Stderr, "  %s\n", res.ErrorMsg)
		}
		os.Exit(1)
	}
	log.LogStructured("firewall.remote_allowed", map[string]any{
		"url":    url,
		"status": res.Status,
		"count":  len(pkgs),
	}, CodeModuleBlocked)
	return nil
}

// PromptTrust dual-channel (console + desktop MessageBox + toast). Either Yes advances.
type PromptTrust struct {
	Module string `arg:"" name:"module" help:"Module / command name that changed."`
}

func (p *PromptTrust) Run() error {
	name := strings.TrimSpace(p.Module)
	if name == "" {
		return fmt.Errorf("module name required")
	}
	msg := fmt.Sprintf("Untrusted module '%s' changed after running. Approve and reshim?", name)
	_ = notify.Send(settings.AppId, "NVM Firewall", msg+" Answer Yes in the dialog or type y in the console.")

	result := make(chan bool, 2)

	go func() {
		fmt.Printf("%s [y/N]: ", msg)
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			result <- false
			return
		}
		line = strings.TrimSpace(line)
		result <- len(line) > 0 && (line[0] == 'y' || line[0] == 'Y')
	}()

	go func() {
		result <- messageBoxYesNo("NVM Firewall", msg)
	}()

	ok := <-result
	if ok {
		log.LogStructured("firewall.trust_prompt_accepted", map[string]any{"module": name}, CodePolicyMutate)
		return nil
	}
	log.LogStructured("firewall.trust_prompt_declined", map[string]any{"module": name}, CodePolicyMutate)
	os.Exit(1)
	return nil
}

func messageBoxYesNo(title, body string) bool {
	user32 := windows.NewLazySystemDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, err1 := syscall.UTF16PtrFromString(title)
	b, err2 := syscall.UTF16PtrFromString(body)
	if err1 != nil || err2 != nil {
		return false
	}
	const mbYesNo = 0x00000004
	const mbIconQuestion = 0x00000020
	const mbSetForeground = 0x00010000
	const mbTopmost = 0x00040000
	const idYes = 6
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)), uintptr(mbYesNo|mbIconQuestion|mbSetForeground|mbTopmost))
	return r == idYes
}
