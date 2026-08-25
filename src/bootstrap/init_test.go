package bootstrap

import (
	"common/fs"
	prefs "common/preferences"
	"common/registry"
	"common/settings"
	"common/verifycache"
	"errors"
	"fmt"
	"nvm/link"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	winreg "golang.org/x/sys/windows/registry"
)

const bootstrapTestRegistryRoot = "HKCU/Software/NVMTest/bootstrap_test"

func TestMain(m *testing.M) {
	prefs.ROOT = bootstrapTestRegistryRoot
	prefs.ROOTS = []string{prefs.ROOT}
	code := m.Run()
	exec.Command("reg", "delete", `HKCU\Software\NVMTest`, "/f").Run() //nolint:errcheck
	os.Exit(code)
}

func unlockShimLayoutForTestCleanup(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		shimDir, err := ShimDir()
		if err == nil {
			_ = fs.UnlockShimDirectory(shimDir)
		}
		proxyPath, err := DataProxyPath()
		if err == nil {
			_ = fs.UnlockProxyExecutable(proxyPath)
		}
	})
}

func TestEnsureUserProfileInitializedCreatesShimModeLayout(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	assertPathExists(t, filepath.Join(root, ".shim"))
	assertPathExists(t, filepath.Join(root, ".link"))
	assertPathExists(t, filepath.Join(root, ".nodejs"))
	assertPathExists(t, filepath.Join(root, ".verify"))
	assertPathExists(t, verifycache.PubKeyPath(root))
	assertBootstrapVersion(t, currentBootstrapVersion)
	if _, err := os.Stat(filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Stat(.nodejs) error = %v", err)
	}
}

func TestEnsureUserProfileInitializedCreatesLinkModeLayout(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "installs")
	versionDir := filepath.Join(installRoot, "v22.0.0")
	resetBootstrapState(t)
	stubActivationVerification(t)

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("MkdirAll(versionDir) error = %v", err)
	}
	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "link"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}
	if err := settings.Put("active_version", "22.0.0"); err != nil {
		t.Fatalf("Put(active_version) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	assertPathExists(t, filepath.Join(root, ".shim"))
	assertPathExists(t, filepath.Join(root, ".link"))
	assertPathExists(t, filepath.Join(root, ".link", "nodejs"))
	assertPathExists(t, filepath.Join(root, ".nodejs"))
	assertBootstrapVersion(t, currentBootstrapVersion)
	if _, err := os.Stat(filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Stat(.nodejs) error = %v", err)
	}
}

func TestEnsureUserProfileInitializedRepairsWrongNodejsTarget(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}
	if err := settings.Put("enabled", true); err != nil {
		t.Fatalf("Put(enabled) error = %v", err)
	}

	if err := EnsureHiddenDir(filepath.Join(root, ".shim")); err != nil {
		t.Fatalf("EnsureHiddenDir(.shim) error = %v", err)
	}
	if err := EnsureHiddenDir(filepath.Join(root, ".link")); err != nil {
		t.Fatalf("EnsureHiddenDir(.link) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".shim", "node.exe"), []byte("node-shim-stub"), 0o644); err != nil {
		t.Fatalf("WriteFile(.shim\\node.exe) error = %v", err)
	}
	if err := link.Link(filepath.Join(root, ".link"), filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Link(.link -> .nodejs) error = %v", err)
	}
	if err := registry.Put(currentBootstrapVersion, bootstrapVersionPath()); err != nil {
		t.Fatalf("Put(bootstrap version) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	targetProbe := filepath.Join(root, ".shim", "node.exe")
	linkProbe := filepath.Join(root, ".nodejs", "node.exe")
	targetInfo, err := os.Lstat(targetProbe)
	if err != nil {
		t.Fatalf("Lstat(.shim\\node.exe) error = %v", err)
	}
	linkInfo, err := os.Lstat(linkProbe)
	if err != nil {
		t.Fatalf("Lstat(.nodejs\\node.exe) error = %v", err)
	}
	if !os.SameFile(targetInfo, linkInfo) {
		t.Fatalf(".nodejs does not resolve to .shim")
	}
}

func TestEnsureUserProfileInitializedRepairsMissingNodejsAfterMarkerSet(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() initial error = %v", err)
	}

	if err := os.Remove(filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Remove(.nodejs) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() repair error = %v", err)
	}

	assertPathExists(t, filepath.Join(root, ".nodejs"))
	assertBootstrapVersion(t, currentBootstrapVersion)
}

func TestEnsureUserProfileInitializedRepairsMissingLinkArtifactsAfterMarkerSet(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "installs")
	versionDir := filepath.Join(installRoot, "v22.0.0")
	resetBootstrapState(t)
	stubActivationVerification(t)

	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("MkdirAll(versionDir) error = %v", err)
	}
	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "link"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}
	if err := settings.Put("active_version", "22.0.0"); err != nil {
		t.Fatalf("Put(active_version) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() initial error = %v", err)
	}

	if err := os.Remove(filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Remove(.nodejs) error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".link", "nodejs")); err != nil {
		t.Fatalf("Remove(.link\\nodejs) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() repair error = %v", err)
	}

	assertPathExists(t, filepath.Join(root, ".link", "nodejs"))
	assertPathExists(t, filepath.Join(root, ".nodejs"))
	assertBootstrapVersion(t, currentBootstrapVersion)
}

func TestEnsureUserProfileInitializedMigratesLegacyMarkerWithoutRepair(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	if err := EnsureHiddenDir(filepath.Join(root, ".shim")); err != nil {
		t.Fatalf("EnsureHiddenDir(.shim) error = %v", err)
	}
	if err := EnsureHiddenDir(filepath.Join(root, ".link")); err != nil {
		t.Fatalf("EnsureHiddenDir(.link) error = %v", err)
	}
	if err := link.Link(filepath.Join(root, ".shim"), filepath.Join(root, ".nodejs")); err != nil {
		t.Fatalf("Link(.shim -> .nodejs) error = %v", err)
	}
	if err := registry.Put(uint32(1), legacyInitializationMarkerPath()); err != nil {
		t.Fatalf("Put(legacy initialization marker) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() migration error = %v", err)
	}

	assertBootstrapVersion(t, currentBootstrapVersion)
	assertLegacyMarkerRemoved(t)
}

func TestEnsureUserProfileInitializedSeedsMissingSyncAssets(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)
	overrideLegacyMigrationTargets(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	sourcePath := createProgramSyncSeed(t, filepath.Join(testPathPart(t), "checks", "seed.dll"), []byte("seed-sync"))
	targetPath := filepath.Join(root, ".sync", filepath.Base(filepath.Dir(filepath.Dir(sourcePath))), "checks", "seed.dll")

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	assertFileContent(t, targetPath, []byte("seed-sync"))
}

func TestEnsureUserProfileInitializedPreservesExistingSyncAssets(t *testing.T) {
	root := t.TempDir()
	resetBootstrapState(t)
	overrideLegacyMigrationTargets(t)

	if err := settings.Put("root", filepath.Join(root, "installs")); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	relPath := filepath.Join(testPathPart(t), "news.dll")
	_ = createProgramSyncSeed(t, relPath, []byte("seed-version"))
	targetPath := filepath.Join(root, ".sync", relPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("MkdirAll(targetPath) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("user-version"), 0644); err != nil {
		t.Fatalf("WriteFile(targetPath) error = %v", err)
	}

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	assertFileContent(t, targetPath, []byte("user-version"))
}

func TestEnsureUserProfileInitializedCleansLegacyPayload(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "installs")
	resetBootstrapState(t)
	overrideLegacyMigrationTargets(t)

	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}
	if err := settings.Put("mode", "shim"); err != nil {
		t.Fatalf("Put(mode) error = %v", err)
	}

	legacyNvmExe := filepath.Join(root, "nvm.exe")
	legacySyncExe := filepath.Join(root, "utils", "sync.exe")
	if err := os.MkdirAll(filepath.Dir(legacySyncExe), 0755); err != nil {
		t.Fatalf("MkdirAll(legacySyncExe) error = %v", err)
	}
	if err := os.WriteFile(legacyNvmExe, []byte("legacy-nvm"), 0644); err != nil {
		t.Fatalf("WriteFile(legacyNvmExe) error = %v", err)
	}
	if err := os.WriteFile(legacySyncExe, []byte("legacy-sync"), 0644); err != nil {
		t.Fatalf("WriteFile(legacySyncExe) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".icons"), 0755); err != nil {
		t.Fatalf("MkdirAll(.icons) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".icons", "nvm.ico"), []byte("legacy-icon"), 0644); err != nil {
		t.Fatalf("WriteFile(.icons\\nvm.ico) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".sync"), 0755); err != nil {
		t.Fatalf("MkdirAll(.sync) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".sync", "keep.dll"), []byte("keep"), 0644); err != nil {
		t.Fatalf("WriteFile(.sync\\keep.dll) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(installRoot, "v22.0.0"), 0755); err != nil {
		t.Fatalf("MkdirAll(installRoot) error = %v", err)
	}

	deletedTasks := []string{}
	legacySyncTaskNames = []string{"NVM for Windows Sync", "NVM Sync"}
	queryScheduledTaskXML = func(taskName string) (string, error) {
		if taskName == "NVM Sync" {
			return "<Exec><Command>" + legacySyncExe + "</Command></Exec>", nil
		}
		return "", errors.New("ERROR: The system cannot find the file specified.")
	}
	deleteScheduledTask = func(taskName string) error {
		deletedTasks = append(deletedTasks, taskName)
		return nil
	}

	createRegistryKey(t, legacyUserUninstallKeyPaths[0], map[string]string{
		"InstallLocation": root,
		"UninstallString": `"` + filepath.Join(root, "unins000.exe") + `"`,
	})
	createRegistryKey(t, legacyNvmAppPathKey, map[string]string{
		"": legacyNvmExe,
	})
	createRegistryKey(t, legacySyncAppPathKey, map[string]string{
		"": legacySyncExe,
	})
	createRegistryKey(t, legacyShellRegistrationBase+`\shell\open\command`, map[string]string{
		"": `"` + legacyNvmExe + `" "%1"`,
	})
	createRegistryKey(t, `Environment`, map[string]string{
		"NVM_HOME": root,
	})

	if err := EnsureUserProfileInitialized(); err != nil {
		t.Fatalf("EnsureUserProfileInitialized() error = %v", err)
	}

	assertPathMissing(t, legacyNvmExe)
	assertPathMissing(t, filepath.Join(root, "utils"))
	assertPathMissing(t, filepath.Join(root, ".icons"))
	assertPathExists(t, filepath.Join(root, ".sync", "keep.dll"))
	assertPathExists(t, filepath.Join(installRoot, "v22.0.0"))
	assertRegistryKeyMissing(t, legacyUserUninstallKeyPaths[0])
	assertRegistryKeyMissing(t, legacyNvmAppPathKey)
	assertRegistryKeyMissing(t, legacySyncAppPathKey)
	assertRegistryKeyMissing(t, legacyShellRegistrationBase+`\shell\open\command`)
	assertRegistryValueMissing(t, `Environment`, "NVM_HOME")
	if len(deletedTasks) != 1 || deletedTasks[0] != "NVM Sync" {
		t.Fatalf("deleted tasks = %#v, want [\"NVM Sync\"]", deletedTasks)
	}
	assertBootstrapVersion(t, currentBootstrapVersion)
}

func TestCleanupLegacyUserPayloadSkipsLivePerUserInstall(t *testing.T) {
	root := t.TempDir()
	installRoot := filepath.Join(root, "installs")
	resetBootstrapState(t)
	overrideLegacyMigrationTargets(t)

	oldProgramRoot := programRootOverride
	programRootOverride = root
	t.Cleanup(func() { programRootOverride = oldProgramRoot })

	if err := settings.Put("root", installRoot); err != nil {
		t.Fatalf("Put(root) error = %v", err)
	}

	liveNvmExe := filepath.Join(root, "nvm.exe")
	liveSyncExe := filepath.Join(root, "utils", "sync.exe")
	if err := os.MkdirAll(filepath.Dir(liveSyncExe), 0755); err != nil {
		t.Fatalf("MkdirAll(liveSyncExe) error = %v", err)
	}
	if err := os.WriteFile(liveNvmExe, []byte("live-nvm"), 0644); err != nil {
		t.Fatalf("WriteFile(liveNvmExe) error = %v", err)
	}
	if err := os.WriteFile(liveSyncExe, []byte("live-sync"), 0644); err != nil {
		t.Fatalf("WriteFile(liveSyncExe) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".icons"), 0755); err != nil {
		t.Fatalf("MkdirAll(.icons) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".icons", "nvm.ico"), []byte("icon"), 0644); err != nil {
		t.Fatalf("WriteFile(icon) error = %v", err)
	}

	deletedTasks := 0
	queryScheduledTaskXML = func(taskName string) (string, error) {
		return "<Exec><Command>" + liveSyncExe + "</Command></Exec>", nil
	}
	deleteScheduledTask = func(taskName string) error {
		deletedTasks++
		return nil
	}

	createRegistryKey(t, legacyUserUninstallKeyPaths[0], map[string]string{
		"InstallLocation": root,
	})
	createRegistryKey(t, `Environment`, map[string]string{
		"NVM_HOME": root,
	})

	if err := cleanupLegacyUserPayload(root); err != nil {
		t.Fatalf("cleanupLegacyUserPayload() error = %v", err)
	}

	assertPathExists(t, liveNvmExe)
	assertPathExists(t, liveSyncExe)
	assertPathExists(t, filepath.Join(root, ".icons", "nvm.ico"))
	assertRegistryValueMissing(t, `Environment`, "NVM_HOME")
	if _, err := winreg.OpenKey(winreg.CURRENT_USER, legacyUserUninstallKeyPaths[0], winreg.QUERY_VALUE); err != nil {
		t.Fatalf("live uninstall key should remain, OpenKey error = %v", err)
	}
	if deletedTasks != 0 {
		t.Fatalf("deletedTasks = %d, want 0 for live per-user install", deletedTasks)
	}
}

func TestRemoveLegacyPathSkipsAccessDenied(t *testing.T) {
	err := fmt.Errorf("unlinkat C:\\x\\nvm.exe: Access is denied.")
	if !isBusyLegacyPathError(err) {
		t.Fatalf("isBusyLegacyPathError(%v) = false, want true", err)
	}
}

func TestSyncShimExecutableCopiesMissingTarget(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "node.exe")
	targetPath := filepath.Join(targetDir, "node.exe")
	content := []byte("shim-source")

	if err := os.WriteFile(sourcePath, content, 0644); err != nil {
		t.Fatalf("WriteFile(sourcePath) error = %v", err)
	}

	if err := syncShimExecutable(sourcePath, targetPath); err != nil {
		t.Fatalf("syncShimExecutable() error = %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(targetPath) error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("target content = %q, want %q", got, content)
	}
}

func TestSyncShimExecutableReplacesStaleTarget(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "node.exe")
	targetPath := filepath.Join(targetDir, "node.exe")

	if err := os.WriteFile(targetPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile(targetPath) error = %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("fresh-shim"), 0644); err != nil {
		t.Fatalf("WriteFile(sourcePath) error = %v", err)
	}

	if err := syncShimExecutable(sourcePath, targetPath); err != nil {
		t.Fatalf("syncShimExecutable() error = %v", err)
	}

	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(targetPath) error = %v", err)
	}
	if string(got) != "fresh-shim" {
		t.Fatalf("target content = %q, want %q", got, "fresh-shim")
	}
}

func TestSyncShimExecutableSkipsMissingSource(t *testing.T) {
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "node.exe")

	if err := syncShimExecutable(filepath.Join(targetDir, "missing-node.exe"), targetPath); err != nil {
		t.Fatalf("syncShimExecutable() error = %v", err)
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("Stat(targetPath) error = %v, want not exists", err)
	}
}

func TestSyncSharedExecutablePreservesHardLinks(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "proxy.exe")
	targetPath := filepath.Join(targetDir, "proxy.exe")
	linkedPath := filepath.Join(targetDir, "npm.exe")

	if err := os.WriteFile(sourcePath, []byte("proxy-v1"), 0644); err != nil {
		t.Fatalf("WriteFile(sourcePath) error = %v", err)
	}
	if err := syncSharedExecutable(sourcePath, targetPath); err != nil {
		t.Fatalf("syncSharedExecutable() initial error = %v", err)
	}
	if err := os.Link(targetPath, linkedPath); err != nil {
		t.Fatalf("Link(targetPath, linkedPath) error = %v", err)
	}

	if err := os.WriteFile(sourcePath, []byte("proxy-v2"), 0644); err != nil {
		t.Fatalf("WriteFile(sourcePath) update error = %v", err)
	}
	if err := syncSharedExecutable(sourcePath, targetPath); err != nil {
		t.Fatalf("syncSharedExecutable() update error = %v", err)
	}

	got, err := os.ReadFile(linkedPath)
	if err != nil {
		t.Fatalf("ReadFile(linkedPath) error = %v", err)
	}
	if string(got) != "proxy-v2" {
		t.Fatalf("linked content = %q, want %q", got, "proxy-v2")
	}
}

func TestSyncSharedExecutableSkipsMissingSource(t *testing.T) {
	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "proxy.exe")

	if err := syncSharedExecutable(filepath.Join(targetDir, "missing-proxy.exe"), targetPath); err != nil {
		t.Fatalf("syncSharedExecutable() error = %v", err)
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("Stat(targetPath) error = %v, want not exists", err)
	}
}

func resetBootstrapState(t *testing.T) {
	t.Helper()
	unlockShimLayoutForTestCleanup(t)
	if err := settings.Del("root"); err != nil {
		t.Fatalf("Del(root) error = %v", err)
	}
	if err := settings.Del("mode"); err != nil {
		t.Fatalf("Del(mode) error = %v", err)
	}
	if err := settings.Del("active_version"); err != nil {
		t.Fatalf("Del(active_version) error = %v", err)
	}
	if err := registry.Del(bootstrapVersionPath()); err != nil {
		t.Fatalf("Del(bootstrap version marker) error = %v", err)
	}
	if err := registry.Del(legacyInitializationMarkerPath()); err != nil {
		t.Fatalf("Del(legacy initialization marker) error = %v", err)
	}
}

func stubActivationVerification(t *testing.T) {
	t.Helper()
	originalVerify := verifyActivationNode
	verifyActivationNode = func(string) error { return nil }
	t.Cleanup(func() { verifyActivationNode = originalVerify })
}

func assertBootstrapVersion(t *testing.T, want uint32) {
	t.Helper()

	value, exists, err := registry.Get(bootstrapVersionPath())
	if err != nil {
		t.Fatalf("Get(bootstrap version marker) error = %v", err)
	}
	if !exists {
		t.Fatalf("bootstrap version marker does not exist")
	}

	got, err := normalizeBootstrapVersion(value)
	if err != nil {
		t.Fatalf("normalizeBootstrapVersion(%v) error = %v", value, err)
	}
	if got != want {
		t.Fatalf("bootstrap version = %d, want %d", got, want)
	}
}

func assertLegacyMarkerRemoved(t *testing.T) {
	t.Helper()

	_, exists, err := registry.GetBool(legacyInitializationMarkerPath())
	if err != nil {
		t.Fatalf("GetBool(legacy initialization marker) error = %v", err)
	}
	if exists {
		t.Fatalf("legacy initialization marker still exists")
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("Lstat(%q) error = %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("Lstat(%q) error = %v, want not exists", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertRegistryKeyMissing(t *testing.T, keyPath string) {
	t.Helper()

	key, err := winreg.OpenKey(winreg.CURRENT_USER, keyPath, winreg.QUERY_VALUE)
	if err == nil {
		key.Close()
		t.Fatalf("registry key %q still exists", keyPath)
	}
	if err != winreg.ErrNotExist {
		t.Fatalf("OpenKey(%q) error = %v, want %v", keyPath, err, winreg.ErrNotExist)
	}
}

func assertRegistryValueMissing(t *testing.T, keyPath, valueName string) {
	t.Helper()

	key, err := winreg.OpenKey(winreg.CURRENT_USER, keyPath, winreg.QUERY_VALUE)
	if err != nil {
		t.Fatalf("OpenKey(%q) error = %v", keyPath, err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(valueName)
	if err != winreg.ErrNotExist {
		t.Fatalf("GetStringValue(%q, %q) error = %v, want %v", keyPath, valueName, err, winreg.ErrNotExist)
	}
}

func createProgramSyncSeed(t *testing.T, relPath string, content []byte) string {
	t.Helper()

	programRoot, err := ProgramRoot()
	if err != nil {
		t.Fatalf("ProgramRoot() error = %v", err)
	}

	path := filepath.Join(programRoot, ".sync", relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(programRoot, ".sync", testPathPart(t)))
	})

	return path
}

func createRegistryKey(t *testing.T, keyPath string, values map[string]string) {
	t.Helper()

	key, _, err := winreg.CreateKey(winreg.CURRENT_USER, keyPath, winreg.SET_VALUE)
	if err != nil {
		t.Fatalf("CreateKey(%q) error = %v", keyPath, err)
	}
	defer key.Close()

	for name, value := range values {
		if err := key.SetStringValue(name, value); err != nil {
			t.Fatalf("SetStringValue(%q, %q) error = %v", keyPath, name, err)
		}
	}
}

func overrideLegacyMigrationTargets(t *testing.T) {
	t.Helper()

	keyRoot := `Software\NVMTest\bootstrap_test\` + strings.ReplaceAll(testPathPart(t), string(filepath.Separator), `_`)
	oldUninstallKeys := append([]string(nil), legacyUserUninstallKeyPaths...)
	oldNvmAppPathKey := legacyNvmAppPathKey
	oldSyncAppPathKey := legacySyncAppPathKey
	oldShellBase := legacyShellRegistrationBase
	oldTaskNames := append([]string(nil), legacySyncTaskNames...)
	oldQueryTask := queryScheduledTaskXML
	oldDeleteTask := deleteScheduledTask

	legacyUserUninstallKeyPaths = []string{
		keyRoot + `\Uninstall\40078385-F676-4C61-9A9C-F9028599D6D3_is1`,
		keyRoot + `\Uninstall\nvm_is1`,
	}
	legacyNvmAppPathKey = keyRoot + `\App Paths\nvm.exe`
	legacySyncAppPathKey = keyRoot + `\App Paths\sync.exe`
	legacyShellRegistrationBase = keyRoot + `\Classes\nvm`
	legacySyncTaskNames = []string{"NVM for Windows Sync", "NVM Sync"}
	queryScheduledTaskXML = defaultScheduledTaskQuery
	deleteScheduledTask = defaultScheduledTaskDelete

	t.Cleanup(func() {
		legacyUserUninstallKeyPaths = oldUninstallKeys
		legacyNvmAppPathKey = oldNvmAppPathKey
		legacySyncAppPathKey = oldSyncAppPathKey
		legacyShellRegistrationBase = oldShellBase
		legacySyncTaskNames = oldTaskNames
		queryScheduledTaskXML = oldQueryTask
		deleteScheduledTask = oldDeleteTask
	})
}

func testPathPart(t *testing.T) string {
	t.Helper()
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(t.Name())
}
