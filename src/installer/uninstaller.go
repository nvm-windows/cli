package installer

import (
	"common/resolver"
	"common/settings"
	"fmt"
	"nvm/log"
	"os"
	"path/filepath"
	"runtime"
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
	Range      bool
}

func Uninstall(cfg UninstallConfig) error {
	if len(cfg.Versions) == 0 {
		return nil
	}

	defer updateSystemVersions()
	defer healInstalledVersionVisibility()

	var wg sync.WaitGroup
	var failures atomic.Int32
	var skipped atomic.Int32

	targets := cfg.Versions
	if cfg.Range {
		targets = expandUninstallTargets(cfg.Versions, &skipped)
	}

	targetSet := make(map[string]struct{}, len(targets))
	for _, version := range targets {
		normalized := normalizeVersionSpec(version)
		if normalized != "" {
			targetSet[normalized] = struct{}{}
		}
	}
	if err := prepareActiveForUninstall(targetSet); err != nil {
		return err
	}

	dedupe := map[string]bool{}
	for _, version := range targets {
		if _, seen := dedupe[version]; !seen {
			wg.Add(1)
			go uninstallVersion(version, cfg, &wg, &failures)
			dedupe[version] = true
		}
	}

	wg.Wait()

	if failures.Load() > 0 {
		return fmt.Errorf("%d version(s) failed to uninstall", failures.Load())
	}

	if len(dedupe) == 0 && skipped.Load() > 0 {
		return nil
	}

	return reshim()
}

func prepareActiveForUninstall(targetSet map[string]struct{}) error {
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
				fmt.Printf("Switching default version from v%s to v%s before uninstall.\n", active, last)
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
		fmt.Printf("Switching active version from v%s to v%s before uninstall.\n", active, fallback)
		return nil
	}

	if err := settings.Put("active_version", ""); err != nil {
		return err
	}
	if err := settings.Put("last_version", ""); err != nil {
		return err
	}
	fmt.Printf("Clearing active version v%s before uninstall (no fallback installed).\n", active)
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

func expandUninstallTargets(inputs []string, skipped *atomic.Int32) []string {
	installed := scanInstalledVersions()
	if len(installed) == 0 {
		targets := make([]string, 0, len(inputs))
		for _, input := range inputs {
			if isRangeSpec(input) {
				fmt.Printf("SKIPPED v%s (no installed versions match range)\n", strings.TrimSpace(input))
				skipped.Add(1)
				continue
			}
			targets = append(targets, input)
		}
		return targets
	}

	targets := make([]string, 0, len(inputs))
	for _, input := range inputs {
		spec := normalizeVersionSpec(input)
		if !isRangeSpec(spec) {
			targets = append(targets, input)
			continue
		}

		matches := matchInstalledRange(spec, installed)
		if len(matches) == 0 {
			fmt.Printf("SKIPPED v%s (no installed versions match range)\n", strings.TrimSpace(input))
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

func uninstallVersion(version string, cfg UninstallConfig, wg *sync.WaitGroup, failures *atomic.Int32) {
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
			fmt.Fprintf(os.Stderr, "FAILED v%s %v\n", version, err)
			failures.Add(1)
			return
		}
		node_version = normalizeVersionSpec(resolvedVersion)
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
	if uninstallDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[uninstall-debug] candidate-dirs=%v\n", installDirs)
	}

	dirExists := len(installDirs) > 0
	for _, installDir := range installDirs {
		if err := os.RemoveAll(installDir); err != nil {
			log.Errorf("Failed to uninstall Node.js v%s: %v", node_version, err)
			fmt.Fprintf(os.Stderr, "FAILED v%s %v\n", node_version, err)
			failures.Add(1)
			return
		}
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
		failures.Add(1)
		return
	}

	keyExists := false
	if k, err := registry.OpenKey(registry.CURRENT_USER, registryKeyName(node_version), registry.QUERY_VALUE); err == nil {
		k.Close()
		keyExists = true
	}
	if keyExists {
		unregisterNodeVersion(node_version)
	}

	if !dirExists && !keyExists {
		fmt.Printf("SKIPPED v%s (not installed)\n", node_version)
		return
	}

	if cfg.ClearCache && cfg.CacheDir != "" {
		cpuarch := runtime.GOARCH
		if cpuarch == "amd64" {
			cpuarch = "x64"
		}
		archiveName := fmt.Sprintf("node-v%s-win-%s.7z", node_version, cpuarch)
		cacheFile := filepath.Join(cfg.CacheDir, archiveName)
		if _, err := os.Stat(cacheFile); err == nil {
			os.Remove(cacheFile)
		}
	}

	fmt.Printf("Uninstalled Node.js v%s\n", node_version)
	log.Logf("Uninstalled Node.js v%s", node_version)
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
	registry.DeleteKey(registry.CURRENT_USER, registryKeyName(version))
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
