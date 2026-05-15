package installer

import (
	"common/fs"
	"common/http"
	"common/resolver"
	"common/settings"
	"common/system"
	"common/version_support"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"

	"common/acl"
	"common/notify"
	"nvm/log"
)

type InstallConfig struct {
	Notify                bool
	Debug                 bool
	Cache                 bool
	CacheOnly             bool
	NoCache               bool
	Force                 bool
	CacheDir              string
	Versions              []string
	AllowInsecure         bool
	CopyModulesFrom       string
	ModulesFrom           string
	AutoInstallModuleList []string
	LocalOnly             bool
}

var (
	installModulesFromFn = installModulesFrom
	copyModulesFn        = copyModules
	autoInstallModulesFn = autoInstallModules
)

func runPostInstallModuleActions(ctx context.Context, cfg InstallConfig, nodeVersion string, status *Status) (bool, error) {
	if cfg.ModulesFrom != "" {
		return true, installModulesFromFn(ctx, cfg.ModulesFrom, nodeVersion, status)
	}

	if cfg.CopyModulesFrom != "" {
		return true, copyModulesFn(ctx, cfg.CopyModulesFrom, nodeVersion, status)
	}

	if len(cfg.AutoInstallModuleList) > 0 {
		return true, autoInstallModulesFn(ctx, cfg.AutoInstallModuleList, nodeVersion, status)
	}

	return false, nil
}

func Install(cfg InstallConfig) error {
	if len(cfg.Versions) == 0 {
		return nil
	}

	var wg sync.WaitGroup

	dedupe := map[string]bool{}
	for _, version := range cfg.Versions {
		if _, seen := dedupe[version]; !seen {
			dedupe[version] = true
		}
	}

	status := newStatus()
	status.Versions = slices.Collect(maps.Keys(dedupe))
	status.Start()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			status.Cancel()
			cancel()
		case <-ctx.Done():
		}
	}()

	txns := make([]*Transaction, 0, len(dedupe))
	var modulesInstalledMu sync.Mutex
	modulesInstalled := false
	index := 0
	for version := range dedupe {
		index++
		txn := &Transaction{}
		txns = append(txns, txn)
		wg.Add(1)
		go install(ctx, version, cfg, &wg, status, index, txn, &modulesInstalledMu, &modulesInstalled)
	}

	wg.Wait()

	if ctx.Err() != nil {
		for _, txn := range txns {
			rollbackCanceledInstall(txn, status)
		}
		return status.Abort("Installation cancelled.", false)
	}

	for _, txn := range txns {
		if txn != nil && txn.installBackup != "" && !txn.backupDiscarded {
			os.RemoveAll(txn.installBackup)
			txn.backupDiscarded = true
		}
	}

	if status.TotalInstalled > 0 || modulesInstalled {
		if err := reshim(); err != nil {
			status.Alert(fmt.Errorf("reshim warning: %v", err))
		}
	}

	status.Done()
	return nil
}

func install(
	ctx context.Context,
	version string,
	cfg InstallConfig,
	wg *sync.WaitGroup,
	status *Status,
	index int,
	txn *Transaction,
	modulesInstalledMu *sync.Mutex,
	modulesInstalled *bool,
) {
	defer wg.Done()

	status.Versions[index-1] = ""

	nodeVersion, _, err := resolver.Find(version)
	if err != nil {
		status.Alert(fmt.Errorf("FAILED v%s: %v", version, err), false)
		return
	}

	if allowed, err := acl.IsAllowedVersion(nodeVersion); !allowed {
		status.Alert(fmt.Errorf("%v", err), false)
		return
	}

	*txn = Transaction{
		version:    nodeVersion,
		installDir: getRoot(nodeVersion),
		cacheFile:  cacheArchivePath(nodeVersion, cfg),
	}
	defer healInstalledVersionVisibility(txn.installDir)

	if info, err := os.Stat(txn.installDir); err == nil && info.IsDir() {
		txn.installed = true
	}
	if txn.cacheFile != "" {
		if _, err := os.Stat(txn.cacheFile); err == nil {
			txn.cached = true
		}
	}

	if !cfg.Force && !cfg.CacheOnly {
		if info, err := os.Stat(txn.installDir); err == nil && info.IsDir() {
			status.Alert(fmt.Errorf("SKIPPED: v%s is already installed", nodeVersion), false)
			return
		}
	}

	if supported, err := version_support.IsSupportedVersion(nodeVersion); !supported {
		status.Alert(fmt.Errorf("FAILED: v%s %v", nodeVersion, err), false)
		return
	}

	var processErr error
	if cfg.Force && txn.installed && !cfg.CacheOnly {
		backupPath := fmt.Sprintf("%s.nvm-rollback-%d", txn.installDir, time.Now().UnixNano())
		if err := os.Rename(txn.installDir, backupPath); err != nil {
			status.Alert(fmt.Errorf("FAILED v%s: could not prepare rollback backup: %w", nodeVersion, err))
			return
		}
		txn.installBackup = backupPath
	}

	if !cfg.CacheOnly {
		if err := os.MkdirAll(txn.installDir, 0755); err != nil {
			if txn.installBackup != "" {
				_ = restoreInstallBackup(txn)
			}
			status.Alert(fmt.Errorf("FAILED: v%s could not prepare install root %s: %w", nodeVersion, txn.installDir, err))
			return
		}
		if !txn.installed {
			txn.installedNew = true
		}
	}

	defer func() {
		if system.IsAppInForeground() {
			return
		}
		if ctx.Err() != nil {
			return
		}
		var title, message string
		if processErr == nil {
			message = fmt.Sprintf("Node.js v%s installed", nodeVersion)
		} else {
			title = fmt.Sprintf("Node.js v%s Installation Failed", nodeVersion)
			message = processErr.Error()
		}
		go func() { _ = notify.Send(settings.AppId, title, message) }()
	}()

	processErr = downloadNode(ctx, nodeVersion, filepath.Dir(txn.installDir), cfg, status, txn)

	if errors.Is(processErr, context.Canceled) || (processErr == nil && ctx.Err() != nil) {
		rollbackCanceledInstall(txn, status)
		processErr = context.Canceled
		return
	}

	if processErr != nil && !errors.Is(processErr, context.Canceled) {
		if txn.installedNew && !txn.installed {
			if err := cleanupInstallDir(txn.installDir); err != nil {
				status.Alert(fmt.Errorf("rollback warning for v%s: failed to remove install dir %s: %v", nodeVersion, txn.installDir, err))
			}
		}
		if txn.installBackup != "" {
			if err := restoreInstallBackup(txn); err != nil {
				status.Alert(fmt.Errorf("rollback warning for v%s: failed to restore backup: %v", nodeVersion, err))
			}
		}
		status.Alert(fmt.Errorf("FAILED v%s: %v", nodeVersion, processErr))
		return
	}

	if processErr == nil {
		var ranModuleActions bool
		ranModuleActions, processErr = runPostInstallModuleActions(ctx, cfg, nodeVersion, status)
		if processErr == nil && ranModuleActions {
			modulesInstalledMu.Lock()
			*modulesInstalled = true
			modulesInstalledMu.Unlock()
		}
	}

	if processErr != nil && !errors.Is(processErr, context.Canceled) {
		if txn.installedNew && !txn.installed {
			if err := cleanupInstallDir(txn.installDir); err != nil {
				status.Alert(fmt.Errorf("rollback warning for v%s: failed to remove install dir %s: %v", nodeVersion, txn.installDir, err))
			}
		}
		if txn.installBackup != "" {
			if err := restoreInstallBackup(txn); err != nil {
				status.Alert(fmt.Errorf("rollback warning for v%s: failed to restore backup: %v", nodeVersion, err))
			}
		}
		status.Alert(fmt.Errorf("FAILED v%s: %v", nodeVersion, processErr))
		return
	}

	status.TotalInstalled++
	status.Versions[index-1] = nodeVersion
}

func downloadNode(ctx context.Context, version, target string, cfg InstallConfig, status *Status, txn *Transaction) (retErr error) {
	var installDir string
	defer func() {
		if retErr != nil {
			if errors.Is(retErr, context.Canceled) {
				cleanupExtractArtifacts(target)
				_ = cleanupInstallDir(installDir)
				return
			}
		}
	}()

	cpuarch := runtime.GOARCH
	if cpuarch == "amd64" {
		cpuarch = "x64"
	}

	archiveName := fmt.Sprintf("node-v%s-win-%s.7z", version, cpuarch)
	archivePath := filepath.Join(target, archiveName)
	installDir = filepath.Join(target, fmt.Sprintf("v%s", version))

	shouldSave := !cfg.NoCache && (cfg.Cache || cfg.CacheOnly || settings.Global().CacheDownloads)
	cacheFile := ""
	if txn != nil {
		cacheFile = txn.cacheFile
	}
	if cacheFile == "" && !cfg.NoCache && cfg.CacheDir != "" {
		if err := os.MkdirAll(cfg.CacheDir, 0755); err == nil {
			cacheFile = filepath.Join(cfg.CacheDir, archiveName)
		}
	}

	fromCache := false
	if cacheFile != "" {
		if _, err := os.Stat(cacheFile); err == nil {
			fromCache = true
			archivePath = cacheFile
			if txn != nil {
				txn.cached = true
			}
		}
	}

	status.TotalExtractions++
	realStart := time.Now()
	var logf *os.File

	if cfg.LocalOnly && !fromCache {
		return fmt.Errorf("Node.js v%s not found in local install directory", version)
	}

	if !fromCache && !cfg.LocalOnly {
		downloaded := false
		var downloadEnd time.Time
		insecure := allowInsecureDownloads(cfg)
		status.Downloads++

		for _, mirror := range settings.Global().NodeMirror {
			shasumPath := filepath.Join(target, fmt.Sprintf("SHASUMS256-v%s-win-%s.txt", version, cpuarch))
			_ = os.Remove(shasumPath)

			if cfg.Debug {
				logPath := filepath.Join(target, fmt.Sprintf("nvm-debug-v%s.log", version))
				if f, err := os.Create(logPath); err == nil {
					logf = f
					_, _ = fmt.Fprintf(logf, "[nvm] Download start: %s\n", realStart.Format(time.RFC3339Nano))
				}
			}

			shasumURI := fmt.Sprintf("%s/v%s/SHASUMS256.txt", mirror, version)
			shasumJob, err := http.Download(shasumURI, http.DownloadConfig{Cache: true, Destination: shasumPath, AllowInsecure: insecure})
			if err != nil {
				continue
			}

			select {
			case <-ctx.Done():
				shasumJob.Cancel()
				return context.Canceled
			case result, ok := <-shasumJob.Result:
				if !ok || result.Error != nil || result.Response == nil || !result.Response.Success {
					_ = os.Remove(shasumPath)
					continue
				}
			}

			uri := fmt.Sprintf("%s/v%s/node-v%s-win-%s.7z", mirror, version, version, cpuarch)
			normalizedURI, err := http.NormalizeURL(uri)
			if err != nil {
				_ = os.Remove(shasumPath)
				continue
			}

			job, err := http.Download(normalizedURI, http.DownloadConfig{Destination: target, AllowInsecure: insecure})
			if err != nil {
				_ = os.Remove(shasumPath)
				continue
			}

			var downloadErr error
		downloadLoop:
			for {
				select {
				case <-ctx.Done():
					job.Cancel()
					downloadErr = context.Canceled
					break downloadLoop
				case _, ok := <-job.Progress:
					if !ok {
						job.Progress = nil
						continue
					}
				case result, ok := <-job.Result:
					if !ok {
						downloadErr = fmt.Errorf("download closed unexpectedly")
						break downloadLoop
					}
					if result.Error != nil || result.Response == nil || !result.Response.Success {
						downloadErr = fmt.Errorf("download error")
						break downloadLoop
					}
					downloadEnd = time.Now()
					break downloadLoop
				}
			}

			if downloadErr != nil {
				_ = os.Remove(shasumPath)
				_ = os.Remove(archivePath)
				if errors.Is(downloadErr, context.Canceled) {
					return context.Canceled
				}
				continue
			}

			verified, err := verifyNodeSHASUM(archivePath, shasumPath)
			_ = os.Remove(shasumPath)
			if err != nil {
				_ = os.Remove(archivePath)
				return fmt.Errorf("unable to verify Node.js archive for v%s: %w", version, err)
			}
			if !verified {
				_ = os.Remove(archivePath)
				return fmt.Errorf("invalid Node.js archive for v%s: SHASUM verification failed", version)
			}

			downloaded = true
			break
		}

		status.Downloads--

		if !downloaded {
			err := fmt.Errorf("Node.js v%s not found on server/mirror", version)
			status.Alert(err)
			return err
		}

		if logf != nil {
			_, _ = fmt.Fprintf(logf, "[nvm] Download end:   %s\n", downloadEnd.Format(time.RFC3339Nano))
		}

		if shouldSave && cacheFile != "" {
			status.TotalCached++
			if err := fs.CopyFile(archivePath, cacheFile); err != nil {
				return err
			}
			if txn != nil && !txn.cached {
				txn.cachedNew = true
			}
		}

		if cfg.CacheOnly {
			_ = os.Remove(archivePath)
		} else {
			defer os.Remove(archivePath)
		}
	}

	if cfg.CacheOnly {
		status.Log("Cached v" + version)
		log.Logf("Cached Node.js v%s at %s", version, cacheFile)
		return nil
	}

	if ctx.Err() != nil {
		return context.Canceled
	}

	if logf != nil {
		_, _ = fmt.Fprintf(logf, "[nvm] Extraction start: %s\n", time.Now().Format(time.RFC3339Nano))
	}

	if err := cleanupInstallDir(installDir); err != nil {
		return err
	}
	if err := extract7z(ctx, archivePath, installDir, status); err != nil {
		return err
	}

	publisher, err := verifyAllowedSigner(filepath.Join(installDir, "node.exe"))
	if err != nil {
		return fmt.Errorf("unable to verify Node.js signer for v%s: %w", version, err)
	}

	fs.EnableInheritance(installDir)
	registerNodeVersion(version, installDir, publisher)
	if txn != nil && !txn.installed {
		txn.installedNew = true
	}

	extractEnd := time.Now()
	if logf != nil {
		_, _ = fmt.Fprintf(logf, "[nvm] Extraction end:   %s\n", extractEnd.Format(time.RFC3339Nano))
		_, _ = fmt.Fprintf(logf, "[nvm] True download+extract elapsed: %s\n", extractEnd.Sub(realStart))
		_ = logf.Close()
	}

	return nil
}

func allowInsecureDownloads(cfg InstallConfig) bool {
	value, err := settings.Get("allow_insecure_downloads")
	if err == nil {
		if b, ok := value.(bool); ok {
			if !b {
				return false
			}
		}
	}
	return cfg.AllowInsecure
}

func getRoot(version string) string {
	target := expand(settings.Global().Root)
	return filepath.Join(target, fmt.Sprintf("v%s", version))
}

func expand(path string) string {
	re := regexp.MustCompile(`%([^%]+)%`)
	return re.ReplaceAllStringFunc(path, func(match string) string {
		varName := match[1 : len(match)-1]
		if value, ok := os.LookupEnv(varName); ok {
			return value
		}
		return match
	})
}

func registryKeyName(version string) string {
	return `Software\Microsoft\Windows\CurrentVersion\Uninstall\nvm4w-node-v` + version
}

func registerNodeVersion(version, installDir, publisher string) {
	nvmExe, err := os.Executable()
	if err != nil {
		return
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyName(version), registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	displayName := "Node.js v" + version
	if lts := resolver.LTSName(version); lts != "" {
		displayName = "Node.js " + lts
	}
	if strings.TrimSpace(publisher) == "" {
		publisher = nodePublisher(filepath.Join(installDir, "node.exe"))
	}

	key.SetStringValue("DisplayName", displayName+" via nvm-windows")
	key.SetStringValue("UninstallString", fmt.Sprintf(`"%s" uninstall %s`, nvmExe, version))
	key.SetStringValue("DisplayVersion", version)
	key.SetStringValue("Publisher", publisher)
	key.SetStringValue("Comments", "Installed and managed by nvm-windows")
	key.SetStringValue("ManagedBy", "nvm-windows")
	key.SetStringValue("DisplayIcon", filepath.Join(installDir, "node.exe"))
	key.SetStringValue("InstallLocation", installDir)
	key.SetDWordValue("NoModify", 1)
	key.SetDWordValue("NoRepair", 1)
}
