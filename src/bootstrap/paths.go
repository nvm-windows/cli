package bootstrap

import (
	"common/fs"
	"common/settings"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	// programRootOverride is test-only. When set, ProgramRoot returns it.
	programRootOverride string
)

func ProgramRoot() (string, error) {
	if programRootOverride != "" {
		return filepath.Clean(programRootOverride), nil
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve program root: %w", err)
	}

	return filepath.Clean(filepath.Dir(exe)), nil
}

func InstallRoot() (string, error) {
	installRoot, err := settingString("root")
	if err != nil {
		return "", fmt.Errorf("failed to resolve install root: %w", err)
	}

	if installRoot == "" {
		return "", fmt.Errorf("install root is empty")
	}

	return filepath.Clean(installRoot), nil
}

func DataRoot() (string, error) {
	installRoot, err := InstallRoot()
	if err != nil {
		return "", err
	}

	return filepath.Dir(installRoot), nil
}

func CacheRoot() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".cache"), nil
}

func UtilsRoot() (string, error) {
	root, err := ProgramRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "utils"), nil
}

func UtilityPath(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("utility name is empty")
	}

	root, err := UtilsRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, name), nil
}

func ShimDir() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".shim"), nil
}

func ProgramShimPath() (string, error) {
	root, err := ProgramRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".shim", "node.exe"), nil
}

func ProgramProxyPath() (string, error) {
	return UtilityPath("proxy.exe")
}

func ProgramSyncRoot() (string, error) {
	root, err := ProgramRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".sync"), nil
}

func LinkDir() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".link"), nil
}

func LinkNodePath() (string, error) {
	linkDir, err := LinkDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(linkDir, "nodejs"), nil
}

func NodejsPath() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".nodejs"), nil
}

func DataProxyPath() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, "proxy.exe"), nil
}

func DataSyncRoot() (string, error) {
	root, err := DataRoot()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".sync"), nil
}

func EnsureHiddenDir(path string) error {
	return fs.EnsureHiddenDirectory(path)
}

func ensureRequiredRuntimeDirs(dataRoot string) error {
	for _, name := range []string{".shim", ".link", ".sync", ".cache", ".verify"} {
		if err := EnsureHiddenDir(filepath.Join(dataRoot, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	for _, sub := range []string{"versions", "http"} {
		if err := EnsureHiddenDir(filepath.Join(dataRoot, ".cache", sub)); err != nil {
			return fmt.Errorf(".cache/%s: %w", sub, err)
		}
	}
	return nil
}

func hideRuntimeDataDirs(dataRoot string) error {
	for _, name := range fs.RuntimeDataDirNames() {
		if name == ".shim" {
			continue
		}
		if err := fs.HideDirectory(filepath.Join(dataRoot, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func settingString(name string) (string, error) {
	value, err := settings.Get(name)
	if err != nil {
		return "", err
	}

	if value == nil {
		return "", nil
	}

	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("setting %q is %T, not string", name, value)
	}

	return strings.TrimSpace(os.ExpandEnv(text)), nil
}

func syncShimExecutable(sourcePath, targetPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect source shim %s: %w", sourcePath, err)
	}

	targetInfo, err := os.Stat(targetPath)
	if err == nil {
		if sourceInfo.Size() == targetInfo.Size() && !sourceInfo.ModTime().After(targetInfo.ModTime()) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect target shim %s: %w", targetPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target shim directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "node-shim-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temporary shim file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to open source shim: %w", err)
	}

	_, copyErr := io.Copy(tempFile, sourceFile)
	closeSourceErr := sourceFile.Close()
	closeTargetErr := tempFile.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to copy source shim: %w", copyErr)
	}
	if closeSourceErr != nil {
		return fmt.Errorf("failed to close source shim: %w", closeSourceErr)
	}
	if closeTargetErr != nil {
		return fmt.Errorf("failed to flush target shim: %w", closeTargetErr)
	}

	if err := os.Chtimes(tempPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("failed to stamp shim modtime: %w", err)
	}

	if err := fs.ReplaceExecutable(tempPath, targetPath); err != nil {
		// Existing target may still have Auth Users RX-only from a prior locked
		// directory inherit; grant a write window and retry once.
		_ = fs.UnlockProxyExecutable(targetPath)
		if retryErr := fs.ReplaceExecutable(tempPath, targetPath); retryErr != nil {
			return fmt.Errorf("failed to install target shim: %w", retryErr)
		}
	}

	return nil
}

func syncSharedExecutable(sourcePath, targetPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect source executable %s: %w", sourcePath, err)
	}

	targetInfo, err := os.Stat(targetPath)
	if err == nil {
		if sourceInfo.Size() == targetInfo.Size() && !sourceInfo.ModTime().After(targetInfo.ModTime()) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect target executable %s: %w", targetPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target executable directory: %w", err)
	}

	// Prefer in-place overwrite so existing module hardlinks keep pointing at proxy.exe.
	if err := syncSharedExecutableInPlace(sourcePath, targetPath, sourceInfo); err == nil {
		return nil
	} else if !isBusyExecutableError(err) {
		return err
	}

	// Running npm/npx/etc. hardlinks lock proxy.exe. Replace via temp, then reshim
	// (MaintainShimDirectory) recreates hardlinks to the new file.
	return syncSharedExecutableReplace(sourcePath, targetPath, sourceInfo)
}

func syncSharedExecutableInPlace(sourcePath, targetPath string, sourceInfo os.FileInfo) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source executable: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to open target executable: %w", err)
	}

	_, copyErr := io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to copy source executable: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to flush target executable: %w", closeErr)
	}

	if err := os.Chmod(targetPath, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set target executable mode: %w", err)
	}

	if err := os.Chtimes(targetPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("failed to stamp executable modtime: %w", err)
	}

	return nil
}

func syncSharedExecutableReplace(sourcePath, targetPath string, sourceInfo os.FileInfo) error {
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "proxy-*.exe")
	if err != nil {
		return fmt.Errorf("failed to create temporary proxy file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to open source executable: %w", err)
	}

	_, copyErr := io.Copy(tempFile, sourceFile)
	closeSourceErr := sourceFile.Close()
	closeTempErr := tempFile.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to copy source executable: %w", copyErr)
	}
	if closeSourceErr != nil {
		return fmt.Errorf("failed to close source executable: %w", closeSourceErr)
	}
	if closeTempErr != nil {
		return fmt.Errorf("failed to flush temporary proxy file: %w", closeTempErr)
	}

	if err := os.Chtimes(tempPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("failed to stamp executable modtime: %w", err)
	}

	if err := fs.ReplaceExecutable(tempPath, targetPath); err != nil {
		_ = fs.UnlockProxyExecutable(targetPath)
		if retryErr := fs.ReplaceExecutable(tempPath, targetPath); retryErr != nil {
			return fmt.Errorf("failed to install target executable: %w", retryErr)
		}
	}

	return nil
}

func syncSharedFile(sourcePath, targetPath string) error {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect source file %s: %w", sourcePath, err)
	}

	targetInfo, err := os.Stat(targetPath)
	if err == nil {
		if targetInfo.IsDir() {
			return fmt.Errorf("target file %s is a directory", targetPath)
		}
		if sourceInfo.Size() == targetInfo.Size() && !sourceInfo.ModTime().After(targetInfo.ModTime()) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect target file %s: %w", targetPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target file directory: %w", err)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to open target file: %w", err)
	}

	_, copyErr := io.Copy(targetFile, sourceFile)
	closeErr := targetFile.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to copy source file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to flush target file: %w", closeErr)
	}

	if err := os.Chmod(targetPath, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set target file mode: %w", err)
	}

	if err := os.Chtimes(targetPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("failed to stamp file modtime: %w", err)
	}

	return nil
}

func seedDirectoryContents(sourceRoot, targetRoot string) error {
	sourceInfo, err := os.Stat(sourceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect source directory %s: %w", sourceRoot, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source seed path %s is not a directory", sourceRoot)
	}

	if err := EnsureHiddenDir(targetRoot); err != nil {
		return fmt.Errorf("failed to create target seed directory: %w", err)
	}

	return filepath.Walk(sourceRoot, func(sourcePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("failed to walk seed path %s: %w", sourcePath, walkErr)
		}

		relPath, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve seed path %s: %w", sourcePath, err)
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(targetRoot, relPath)
		if info.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create target seed directory %s: %w", targetPath, err)
			}
			return nil
		}

		if err := syncSharedFile(sourcePath, targetPath); err != nil {
			return fmt.Errorf("failed to seed file %s: %w", relPath, err)
		}

		return nil
	})
}
