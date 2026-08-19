package cfg

import (
	"common/settings"
	"fmt"
	"nvm/commands/cfg/utility"
	"nvm/log"
	"os"
)

type Del struct {
	Name  string `arg:"" help:"Configuration option to delete/unset. Must be one of: ${cfg_opts}." enum:"${cfg_opts}"`
	Quiet bool   `flag:"quiet" short:"q" help:"Suppress prompt when unsetting a configuration option."`
}

func (d *Del) Run() error {
	return resetSetting(d.Name, true)
}

func resetSetting(name string, showResult bool) error {
	if name == "root" {
		var cmd Set
		value, err := settings.DefaultValue("root")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return err
		}
		cmd.Pairs = []string{fmt.Sprintf(`root="%s"`, value)}
		return cmd.Run()
	}

	auditMessage := ""
	oldValue := ""
	if currentValue, err := settings.Get(name); err == nil {
		oldValue = displayValue(name, currentValue)
		if msg, ok := settings.DeletionAuditMessage(name, currentValue); ok {
			auditMessage = msg
		}
	}

	if err := settings.Del(name); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}

	if auditMessage != "" {
		log.Log(auditMessage)
	}

	payload := log.StructuredPayload{
		"Action":        "Modified",
		"Configuration": name,
		"Value":         "(default)",
		"User":          log.Actor(),
	}
	if oldValue != "" {
		payload["Old"] = oldValue
	}
	log.LogStructured("nvm.configuration.changed", payload)

	if !settings.HasChangeAudit(name) {
		log.Logf("%s configuration option reset to default", name)
	}

	if showResult {
		return utility.DisplaySetting(name, "successfully reset %s to %s\n")
	}
	return nil
}
