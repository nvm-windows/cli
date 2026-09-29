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

func warnStartupAdvisoriesIfNeeded() {
	if license.IsCommunityBuild() {
		warnCommunityProgramRootIfNeeded()
		return
	}
	warnCertifiedCommunityFeatureModeIfNeeded()
}

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

func warnCertifiedCommunityFeatureModeIfNeeded() {
	if !license.InCommunityFeatureMode() {
		return
	}
	msg := license.CommunityFeatureModeWarning()
	fmt.Fprintln(os.Stderr, msg)

	root, err := bootstrap.ProgramRoot()
	if err != nil {
		return
	}
	stamp := filepath.Join(root, ".cache", "community-feature-mode-warn.stamp")
	if _, err := eventlog.WriteApplicationWarningThrottled(
		uint32(license.FeatureModeWarnEventID),
		msg,
		stamp,
		time.Hour,
	); err != nil {
		_ = err
	}
}
