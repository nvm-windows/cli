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
	if d.Name == "root" {
		var cmd Set
		if value, err := settings.DefaultValue("root"); err == nil {
			cmd.Pairs = []string{fmt.Sprintf(`root="%s"`, value)}

			return cmd.Run()
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
		}
	}

	auditMessage := ""
	if currentValue, err := settings.Get(d.Name); err == nil {
		if msg, ok := settings.DeletionAuditMessage(d.Name, currentValue); ok {
			auditMessage = msg
		}
	}

	if err := settings.Del(d.Name); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return err
	}

	if auditMessage != "" {
		log.Log(auditMessage)
	}

	if !settings.HasChangeAudit(d.Name) {
		log.Logf("%s configuration option reset to default", d.Name)
	}

	return utility.DisplaySetting(d.Name, "successfully reset %s to %s\n")
}
