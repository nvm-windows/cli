package installer

import (
	"common/notify"
	"common/resolver"
	"common/settings"
	"common/system"
	"common/verifycache"
	"fmt"
	"nvm/log"
	"nvm/prompt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows/registry"
)

type UninstallConfig struct {
	Versions   []string
	ClearCache bool
	CacheDir   string
	// FromApps is set when Windows Apps / ARP QuietUninstallString invokes uninstall.
	// Forces non-interactive behavior and prefers ARP InstallLocation when resolving dirs.
	FromApps bool
}

func Uninstall(cfg UninstallConfig) error {
	if len(cfg.Versions) == 0 {
		return nil
	}

	if err := validateWildcardSpecs(cfg.Versions); err != nil {
		return err
	}

	defer updateSystemVersions()
	defer healInstalledVersionVisibility()

	// Check if user is trying to uninstall all versions
	if len(cfg.Versions) == 1 {
		spec := strings.TrimSpace(cfg.Versions[0])
		if spec == "*" || strings.EqualFold(spec, "all") {
			if cfg.FromApps {
				return fmt.Errorf("windows Apps uninstall does not support removing all versions")
			}
			confirmed, err := prompt.Confirm("WARNING: This will remove all versions of Node.js from your system. This action is irreversible. Continue?", "n")
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
		}
	}

	var notifyMu sync.Mutex
	var notifyMsgs []string
	record := func(msg string) {
		fmt.Print(msg)
		notifyMu.Lock()
		notifyMsgs = append(notifyMsgs, strings.TrimRight(msg, "\n"))
		notifyMu.Unlock()
	}

	var wg sync.WaitGroup
	var failures atomic.Int32
	var skipped atomic.Int32

	// Expand any wildcard targets (e.g., "22.*", "22.1.*")
	targets := expandUninstallTargets(cfg.Versions, &skipped, record)

	targetSet := make(map[string]struct{}, len(targets))
	for _, version := range targets {
		normalized := normalizeVersionSpec(version)
		if normalized != "" {
			targetSet[normalized] = struct{}{}
		}
	}
	if err := prepareActiveForUninstall(targetSet, record); err != nil {
		return err
	}

	dedupe := map[string]bool{}
	for _, version := range targets {
		if _, seen := dedupe[version]; !seen {
			wg.Add(1)
			go uninstallVersion(version, cfg, &wg, &failures, record)
			dedupe[version] = true
		}
	}

	wg.Wait()

	if failures.Load() > 0 {
		err := fmt.Errorf("%d version(s) failed to uninstall", failures.Load())
		if cfg.FromApps {
			log.Error(err)
		}
		return err
	}

	if len(dedupe) == 0 && skipped.Load() > 0 {
		return nil
	}

	// Apps launches with no console — always toast so the user gets feedback.
	if len(notifyMsgs) > 0 && (cfg.FromApps || !system.IsAppInForeground()) {
		go notify.Send(settings.AppId, "", strings.Join(notifyMsgs, "; "))
	}

	return runReshim()
}

// runReshim is swapped in tests to avoid requiring a real reshim.exe.
var runReshim = reshim

func prepareActiveForUninstall(targetSet map[string]struct{}, record func(string)) error {
	cfg := settings.Global()
	active := normalizeVersionSpec(cfg.ActiveVersion)
	if active == "" {
		return nil
	}
	if _, removingActive := targetSet[active]; !removingActive {
		return nil
	}

	installed := scanInstalledVersions()

	last := normalizeVersionSpec(cfg.LastVersion)
	if last != "" {
		if _, removingLast := targetSet[last]; !removingLast {
			if isInstalledInList(installed, last) {
				if err := settings.Put("active_version", last); err != nil {
					return err
				}
				if err := settings.Put("last_version", ""); err != nil {
					return err
				}
				record(fmt.Sprintf("Switching default version from v%s to v%s before uninstall.\n", active, last))
				return nil
			}
		}
	}

	if fallback, ok := nextInstalledFallback(installed, active, targetSet); ok {
		if err := settings.Put("active_version", fallback); err != nil {
			return err
		}
		if err := settings.Put("last_version", ""); err != nil {
			return err
		}
		record(fmt.Sprintf("Switching active version from v%s to v%s before uninstall.\n", active, fallback))
		return nil
	}

	if err := settings.Put("active_version", ""); err != nil {
		return err
	}
	if err := settings.Put("last_version", ""); err != nil {
		return err
	}
	record(fmt.Sprintf("Clearing active version v%s before uninstall (no fallback installed).\n", active))
	return nil
}

func nextInstalledFallback(installed []string, active string, targetSet map[string]struct{}) (string, bool) {
	for _, candidate := range installed {
		normalized := normalizeVersionSpec(candidate)
		if normalized == "" || normalized == active {
			continue
		}
		if _, removing := targetSet[normalized]; removing {
			continue
		}
		return normalized, true
	}
	return "", false
}

func isInstalledInList(installed []string, version string) bool {
	target := normalizeVersionSpec(version)
	if target == "" {
		return false
	}
	for _, candidate := range installed {
		if normalizeVersionSpec(candidate) == target {
			return true
		}
	}
	return false
}

func expandUninstallTargets(inputs []string, skipped *atomic.Int32, record func(string)) []string {
	installed := scanInstalledVersions()
	if len(installed) == 0 {
		targets := make([]string, 0, len(inputs))
		for _, input := range inputs {
			if isWildcardSpec(input) || isRangeSpec(input) {
				record(fmt.Sprintf("SKIPPED v%s (no installed versions match range)\n", strings.TrimSpace(input)))
				skipped.Add(1)
				continue
			}
			targets = append(targets, input)
		}
		return targets
	}

	targets := make([]string, 0, len(inputs))
	for _, input := range inputs {
		// Check for wildcard specs like "22.*" or "22.1.*"
		if isWildcardSpec(input) {
			matches := matchWildcardRange(input, installed)
			if len(matches) == 0 {
				record(fmt.Sprintf("SKIPPED v%s (no installed versions match range)\n", strings.TrimSpace(input)))
				skipped.Add(1)
				continue
			}
			targets = append(targets, matches...)
			continue
		}

		spec := normalizeVersionSpec(input)
		if !isRangeSpec(spec) {
			targets = append(targets, input)
			continue
		}

		matches := matchInstalledRange(spec, installed)
		if len(matches) == 0 {
			record(fmt.Sprintf("SKIPPED v%s (no installed versions match range)\n", strings.TrimSpace(input)))
			skipped.Add(1)
			continue
		}
		targets = append(targets, matches...)
	}
	return targets
}

func normalizeVersionSpec(version string) string {
	return resolver.NormalizeVersion(version)
}

func isRangeSpec(version string) bool {
	parts := strings.Split(normalizeVersionSpec(version), ".")
	if len(parts) == 0 || len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func isWildcardSpec(version string) bool {
	return strings.Contains(version, "*")
}

func validateWildcardSpecs(inputs []string) error {
	for _, input := range inputs {
		raw := strings.TrimSpace(input)
		if raw == "" || !strings.Contains(raw, "*") {
			continue
		}
		if raw == "*" {
			continue
		}

		spec := strings.TrimPrefix(strings.TrimPrefix(raw, "v"), "V")
		parts := strings.Split(spec, ".")

		// Only major.minor.* or major.* are valid wildcard forms.
		if len(parts) != 2 && len(parts) != 3 {
			return invalidWildcardErr(raw)
		}

		if parts[0] == "" || strings.Contains(parts[0], "*") {
			return invalidWildcardErr(raw)
		}
		if _, err := strconv.Atoi(parts[0]); err != nil {
			return invalidWildcardErr(raw)
		}

		if len(parts) == 2 {
			if parts[1] != "*" {
				return invalidWildcardErr(raw)
			}
			continue
		}

		if parts[1] == "" || strings.Contains(parts[1], "*") {
			return invalidWildcardErr(raw)
		}
		if _, err := strconv.Atoi(parts[1]); err != nil {
			return invalidWildcardErr(raw)
		}
		if parts[2] != "*" {
			return invalidWildcardErr(raw)
		}
	}

	return nil
}

func invalidWildcardErr(raw string) error {
	return fmt.Errorf("invalid wildcard version %q", raw)
}

func matchWildcardRange(spec string, installed []string) []string {
	// Remove any leading 'v' or 'V' prefix
	spec = strings.TrimPrefix(strings.TrimPrefix(spec, "v"), "V")

	// Split by dot to parse the version spec
	parts := strings.Split(spec, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nil
	}

	// Check if last part is a wildcard
	if parts[len(parts)-1] != "*" {
		return nil // Not a wildcard spec
	}

	// Build the prefix to match (everything before the wildcard)
	prefix := strings.Join(parts[:len(parts)-1], ".")
	if prefix != "" {
		prefix += "."
	}

	matches := make([]string, 0)
	for _, version := range installed {
		normalized := normalizeVersionSpec(version)
		if strings.HasPrefix(normalized, prefix) {
			matches = append(matches, version)
		}
	}
	return matches
}

func matchInstalledRange(spec string, installed []string) []string {
	parts := strings.Split(normalizeVersionSpec(spec), ".")
	if len(parts) == 0 {
		return nil
	}
	major := parts[0]
	minor := ""
	if len(parts) > 1 {
		minor = parts[1]
	}

	matches := make([]string, 0)
	for _, version := range installed {
		v := normalizeVersionSpec(version)
		vparts := strings.Split(v, ".")
		if len(vparts) < 3 {
			continue
		}
		if vparts[0] != major {
			continue
		}
		if minor != "" && vparts[1] != minor {
			continue
		}
		matches = append(matches, version)
	}
	return matches
}

func uninstallVersion(version string, cfg UninstallConfig, wg *sync.WaitGroup, failures *atomic.Int32, record func(string)) {
	defer wg.Done()

	requestedSpec := normalizeVersionSpec(version)
	node_version := ""

	if isRangeSpec(requestedSpec) {
		if latest, ok := latestInstalledPartialMatch(requestedSpec, scanInstalledVersions()); ok {
			node_version = latest
		}
	}

	if node_version == "" {
		resolvedVersion, _, err := resolver.Find(version)
		if err != nil {
			// Apps uninstall can still proceed from ARP InstallLocation without mirror resolution.
			if cfg.FromApps && lookupARPInstallLocation(version) != "" {
				node_version = normalizeVersionSpec(version)
			} else {
				fmt.Fprintf(os.Stderr, "FAILED v%s %v\n", version, err)
				log.LogSystemChanged("uninstall", version, "", log.OutcomeFailed, err.Error())
				failures.Add(1)
				return
			}
		} else {
			node_version = normalizeVersionSpec(resolvedVersion)
		}
	}

	matchSpecs := []string{normalizeVersionSpec(version), normalizeVersionSpec(node_version)}
	installDirs := findInstalledVersionDirs(matchSpecs...)
	if uninstallDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[uninstall-debug] requested=%s resolved=%s root=%s matches=%v dirs=%v\n", version, node_version, expand(settings.Global().Root), matchSpecs, installDirs)
	}

	// Fall back to legacy expected layout when no on-disk match was found.
	if len(installDirs) == 0 {
		legacy := getRoot(node_version)
		if _, err := os.Stat(legacy); err == nil {
			installDirs = append(installDirs, legacy)
		}
	}
	// Windows Apps may run with a stale/empty Root preference; ARP InstallLocation is authoritative.
	if cfg.FromApps {
		for _, candidate := range []string{node_version, version} {
			if loc := lookupARPInstallLocation(candidate); loc != "" {
				if _, err := os.Stat(loc); err == nil {
					installDirs = appendUniqueInstallDir(installDirs, loc)
				}
			}
		}
	}
	if uninstallDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[uninstall-debug] candidate-dirs=%v\n", installDirs)
	}

	dirExists := len(installDirs) > 0
	for _, installDir := range installDirs {
		npmVersion, _ := installedNpmVersion(installDir)
		_ = verifycache.ClearNodeCache(filepath.Join(installDir, "node.exe"))
		if err := cleanupInstallDir(installDir); err != nil {
			log.Errorf("Failed to uninstall Node.js v%s: %v", node_version, err)
			fmt.Fprintf(os.Stderr, "FAILED v%s %v\n", node_version, err)
			log.LogSystemChanged("uninstall", node_version, installDir, log.OutcomeFailed, err.Error())
			failures.Add(1)
			return
		}

		extras := log.StructuredPayload{}
		if npmVersion != "" {
			extras["NpmVersion"] = npmVersion
		}
		log.LogSystemChanged("uninstall", node_version, installDir, log.OutcomeSucceeded, "", extras)
	}

	// If any matching version folder with node.exe still exists, uninstall did not complete.
	remainingMatches := findInstalledVersionDirs(matchSpecs...)
	if uninstallDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[uninstall-debug] remaining-dirs=%v\n", remainingMatches)
	}
	if len(remainingMatches) > 0 {
		err := fmt.Errorf("matching version directory still contains node.exe after uninstall")
		log.Errorf("Failed to uninstall Node.js v%s: %v", node_version, err)
		fmt.Fprintf(os.Stderr, "FAILED v%s %v\n", node_version, err)
		log.LogSystemChanged("uninstall", node_version, "", log.OutcomeFailed, err.Error())
		failures.Add(1)
		return
	}

	removedRegistryEntry := unregisterNodeVersionForInstallDirs(node_version, installDirs)

	if !dirExists && !removedRegistryEntry {
		record(fmt.Sprintf("SKIPPED v%s (not installed)\n", node_version))
		log.LogSystemChanged("uninstall", node_version, "", log.OutcomeSkipped, "not installed")
		return
	}

	record(fmt.Sprintf("Uninstalled Node.js v%s\n", node_version))
	log.Logf("Uninstalled Node.js v%s", node_version)
	if cfg.ClearCache && cfg.CacheDir != "" {
		if purgeVersionDownloadCache(node_version, cfg.CacheDir) {
			record(fmt.Sprintf("Purged cached artifact for v%s\n", node_version))
			log.Logf("Purged cached artifact for v%s", node_version)
		} else {
			record(fmt.Sprintf("v%s was not in cache; nothing removed from cache\n", node_version))
			log.Logf("v%s was not in cache; nothing removed from cache", node_version)
		}
	}
	if len(installDirs) == 0 && removedRegistryEntry {
		log.LogSystemChanged("uninstall", node_version, "", log.OutcomeSucceeded, "removed registry entry only")
	}
}

func latestInstalledPartialMatch(spec string, installed []string) (string, bool) {
	return resolver.LatestInstalledMatch(spec, installed)
}

func findInstalledVersionDir(version string) (string, bool) {
	dirs := findInstalledVersionDirs(version)
	if len(dirs) == 0 {
		return "", false
	}
	return dirs[0], true
}

func findInstalledVersionDirs(versions ...string) []string {
	root := expand(settings.Global().Root)
	entries, err := os.ReadDir(root)
	if err != nil {
		if uninstallDebugEnabled() {
			fmt.Fprintf(os.Stderr, "[uninstall-debug] read-root-failed root=%s err=%v\n", root, err)
		}
		return nil
	}

	targets := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		n := normalizeVersionSpec(version)
		if n != "" {
			targets[n] = struct{}{}
		}
	}

	result := make([]string, 0, len(targets))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(root, entry.Name())
		nodeExe := filepath.Join(versionDir, "node.exe")
		if _, err := os.Stat(nodeExe); err != nil {
			continue
		}

		if _, ok := targets[normalizeVersionSpec(entry.Name())]; ok {
			result = append(result, versionDir)
		}
	}

	return result
}

func uninstallDebugEnabled() bool {
	v := strings.TrimSpace(os.Getenv("NVM_DEBUG_UNINSTALL"))
	if v == "" {
		return false
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func unregisterNodeVersion(version string) {
	_ = unregisterNodeVersionForInstallDirs(version, nil)
}

func unregisterNodeVersionForInstallDirs(version string, installDirs []string) bool {
	const uninstallRootPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	const keyPrefix = "nvm4w-node-v"

	targetVersion := normalizeVersionSpec(version)
	if targetVersion == "" {
		return false
	}

	removed := false

	normalizedInstallDirs := make(map[string]struct{}, len(installDirs))
	for _, installDir := range installDirs {
		normalized := normalizeInstallPath(installDir)
		if normalized != "" {
			normalizedInstallDirs[normalized] = struct{}{}
		}
	}

	uninstallRoot, err := registry.OpenKey(registry.CURRENT_USER, uninstallRootPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return false
	}
	defer uninstallRoot.Close()

	subKeys, err := uninstallRoot.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}

	for _, subKeyName := range subKeys {
		if !strings.HasPrefix(strings.ToLower(subKeyName), keyPrefix) {
			continue
		}

		fullPath := uninstallRootPath + `\` + subKeyName
		entryKey, err := registry.OpenKey(registry.CURRENT_USER, fullPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		displayVersion, _, _ := entryKey.GetStringValue("DisplayVersion")
		installLocation, _, _ := entryKey.GetStringValue("InstallLocation")
		managedBy, _, _ := entryKey.GetStringValue("ManagedBy")
		entryKey.Close()

		nameVersion := normalizeVersionSpec(strings.TrimPrefix(subKeyName, "nvm4w-node-v"))
		displayVersionNorm := normalizeVersionSpec(displayVersion)
		installLocationNorm := normalizeInstallPath(installLocation)

		matchVersion := nameVersion == targetVersion || displayVersionNorm == targetVersion
		_, matchInstallPath := normalizedInstallDirs[installLocationNorm]
		isManaged := strings.EqualFold(strings.TrimSpace(managedBy), "nvm-windows") || strings.HasPrefix(strings.ToLower(subKeyName), keyPrefix)

		if !isManaged || (!matchVersion && !matchInstallPath) {
			continue
		}

		if err := registry.DeleteKey(registry.CURRENT_USER, fullPath); err == nil {
			removed = true
		}
	}

	return removed
}

func normalizeInstallPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	cleaned := filepath.Clean(trimmed)
	return strings.ToLower(cleaned)
}

func appendUniqueInstallDir(dirs []string, dir string) []string {
	normalized := normalizeInstallPath(dir)
	if normalized == "" {
		return dirs
	}
	for _, existing := range dirs {
		if normalizeInstallPath(existing) == normalized {
			return dirs
		}
	}
	return append(dirs, dir)
}

// lookupARPInstallLocation returns InstallLocation from the per-version Windows Apps ARP entry.
func lookupARPInstallLocation(version string) string {
	target := normalizeVersionSpec(version)
	if target == "" {
		return ""
	}

	keyPath := registryKeyName(target)
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		// Older entries may use a slightly different DisplayVersion spelling; scan managed keys.
		return lookupARPInstallLocationScan(target)
	}
	defer key.Close()

	loc, _, err := key.GetStringValue("InstallLocation")
	if err != nil || strings.TrimSpace(loc) == "" {
		return lookupARPInstallLocationScan(target)
	}
	return filepath.Clean(loc)
}

func lookupARPInstallLocationScan(targetVersion string) string {
	const uninstallRootPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall`
	const keyPrefix = "nvm4w-node-v"

	root, err := registry.OpenKey(registry.CURRENT_USER, uninstallRootPath, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer root.Close()

	subKeys, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return ""
	}

	for _, subKeyName := range subKeys {
		if !strings.HasPrefix(strings.ToLower(subKeyName), keyPrefix) {
			continue
		}
		fullPath := uninstallRootPath + `\` + subKeyName
		entryKey, err := registry.OpenKey(registry.CURRENT_USER, fullPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		displayVersion, _, _ := entryKey.GetStringValue("DisplayVersion")
		installLocation, _, _ := entryKey.GetStringValue("InstallLocation")
		entryKey.Close()

		nameVersion := normalizeVersionSpec(strings.TrimPrefix(subKeyName, "nvm4w-node-v"))
		displayVersionNorm := normalizeVersionSpec(displayVersion)
		if nameVersion != targetVersion && displayVersionNorm != targetVersion {
			continue
		}
		if strings.TrimSpace(installLocation) == "" {
			continue
		}
		return filepath.Clean(installLocation)
	}
	return ""
}

func updateSystemVersions() {
	cfg := settings.Global()
	activeVersion := normalizeVersionSpec(cfg.ActiveVersion)
	lastVersion := normalizeVersionSpec(cfg.LastVersion)

	remaining := scanInstalledVersions()
	installed := make(map[string]struct{}, len(remaining))
	for _, v := range remaining {
		installed[normalizeVersionSpec(v)] = struct{}{}
	}

	if activeVersion != "" {
		if _, ok := installed[activeVersion]; ok {
			if lastVersion != "" {
				if _, ok := installed[lastVersion]; !ok {
					if err := settings.Put("last_version", ""); err != nil {
						log.Error(err)
					}
				}
			}
			return
		}
	}

	if lastVersion != "" {
		if _, ok := installed[lastVersion]; ok {
			if err := settings.Put("active_version", lastVersion); err != nil {
				log.Error(err)
			}
			if err := settings.Put("last_version", ""); err != nil {
				log.Error(err)
			}
			return
		}
	}

	if err := settings.Put("active_version", ""); err != nil {
		log.Error(err)
	}
	if err := settings.Put("last_version", ""); err != nil {
		log.Error(err)
	}

	if len(remaining) == 0 {
		fmt.Println("No Node.js versions remain installed.")
		return
	}

	latest := remaining[0]
	if err := settings.Put("active_version", latest); err != nil {
		log.Error(err)
		return
	}
}

// scanInstalledVersions returns installed versions in descending semver order.
// It also removes empty orphaned version directories as a side effect.
func scanInstalledVersions() []string {
	root := expand(settings.Global().Root)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	// Prune empty orphaned version directories before delegating to resolver.
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		versionDir := filepath.Join(root, entry.Name())
		nodeExe := filepath.Join(versionDir, "node.exe")
		if _, err := os.Stat(nodeExe); os.IsNotExist(err) {
			if dirEntries, readErr := os.ReadDir(versionDir); readErr == nil && len(dirEntries) == 0 {
				if removeErr := os.Remove(versionDir); removeErr != nil {
					log.Errorf("Failed to remove empty version directory %s: %v", versionDir, removeErr)
				}
			}
		}
	}

	return resolver.ScanInstalled()
}
