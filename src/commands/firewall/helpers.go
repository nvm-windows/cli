package firewall

import (
	"bufio"
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
