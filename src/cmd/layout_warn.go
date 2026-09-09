package main

import (
	"common/eventlog"
	"common/license"
	"fmt"
	"nvm/bootstrap"
	"os"
	"path/filepath"
	"time"
)

func warnCommunityProgramRootIfNeeded() {
	root, err := bootstrap.ProgramRoot()
	if err != nil {
		return
	}
	check := license.CheckCommunityProgramRoot(root)
	if check.OK {
		return
	}
	fmt.Fprintln(os.Stderr, check.Message)

	stamp := filepath.Join(root, ".cache", "community-layout-warn.stamp")
	if _, err := eventlog.WriteApplicationWarningThrottled(
		uint32(license.LayoutWarnEventID),
		check.Message,
		stamp,
		time.Hour,
	); err != nil {
		// Best-effort: stderr already carries the advisory.
		_ = err
	}
}

func communityEditionWatermark() string {
	if license.Edition() != "Community" {
		return ""
	}
	return "Community (per-user LocalAppData install; see nvm doctor)"
}
