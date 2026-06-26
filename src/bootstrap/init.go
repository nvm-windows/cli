package bootstrap

import (
	prefs "common/preferences"
	"common/registry"
	"fmt"
	"nvm/link"
	"os"
	"path/filepath"
	"strings"
)

const (
	bootstrapVersionV1                    = uint32(1)
	bootstrapVersionV2                    = uint32(2)
	currentBootstrapVersion               = bootstrapVersionV2
	bootstrapVersionValueName             = "BootstrapVersion"
	legacyUserProfileInitializedValueName = "UserProfileInitialized"
)

func EnsureUserProfileInitialized() error {
	state, err := currentBootstrapState()
	if err != nil {
		return fmt.Errorf("failed to check bootstrap state: %w", err)
	}

	shimDir, err := ShimDir()
	if err != nil {
		return err
	}

	linkDir, err := LinkDir()
	if err != nil {
		return err
	}

	if err := EnsureHiddenDir(shimDir); err != nil {
		return fmt.Errorf("failed to create shim directory: %w", err)
	}

	programShimPath, err := ProgramShimPath()
	if err != nil {
		return err
	}

	if err := syncShimExecutable(programShimPath, filepath.Join(shimDir, "node.exe")); err != nil {
		return fmt.Errorf("failed to synchronize node shim: %w", err)
	}

	programProxyPath, err := ProgramProxyPath()
	if err != nil {
		return err
	}

	dataProxyPath, err := DataProxyPath()
	if err != nil {
		return err
	}

	if err := syncSharedExecutable(programProxyPath, dataProxyPath); err != nil {
		return fmt.Errorf("failed to synchronize shared proxy shim: %w", err)
	}

	programSyncRoot, err := ProgramSyncRoot()
	if err != nil {
		return err
	}

	dataSyncRoot, err := DataSyncRoot()
	if err != nil {
		return err
	}

	if err := seedDirectoryContents(programSyncRoot, dataSyncRoot); err != nil {
		return fmt.Errorf("failed to seed sync assets: %w", err)
	}

	if state.version < currentBootstrapVersion {
		dataRoot, err := DataRoot()
		if err != nil {
			return err
		}

		if err := cleanupLegacyUserPayload(dataRoot); err != nil {
			return fmt.Errorf("failed to clean legacy per-user payload: %w", err)
		}
	}

	if err := EnsureHiddenDir(linkDir); err != nil {
		return fmt.Errorf("failed to create link directory: %w", err)
	}

	nodejsPath, err := NodejsPath()
	if err != nil {
		return err
	}

	mode, err := settingString("mode")
	if err != nil {
		return fmt.Errorf("failed to read operating mode: %w", err)
	}

	if mode == "" {
		mode = "shim"
	}

	activeVersion := ""
	if strings.EqualFold(mode, "link") {
		activeVersion, err = settingString("active_version")
		if err != nil {
			return fmt.Errorf("failed to read active version: %w", err)
		}
	}

	needsRepair, err := profileNeedsRepair(state.version, strings.ToLower(mode), shimDir, linkDir, nodejsPath, dataProxyPath, activeVersion)
	if err != nil {
		return err
	}
	if !needsRepair {
		if state.needsMarkerUpgrade() {
			if err := writeBootstrapVersion(currentBootstrapVersion); err != nil {
				return fmt.Errorf("failed to update bootstrap version marker: %w", err)
			}
		}
		return nil
	}

	switch strings.ToLower(mode) {
	case "shim":
		if err := link.Link(shimDir, nodejsPath); err != nil {
			return fmt.Errorf("failed to initialize .nodejs junction: %w", err)
		}
	case "link":
		linkNodePath, err := LinkNodePath()
		if err != nil {
			return err
		}

		if activeVersion != "" {
			installRoot, err := InstallRoot()
			if err != nil {
				return err
			}

			if err := link.Link(filepath.Join(installRoot, "v"+activeVersion), linkNodePath); err != nil {
				return fmt.Errorf("failed to initialize link-mode target: %w", err)
			}
		}

		if err := link.Link(linkNodePath, nodejsPath); err != nil {
			return fmt.Errorf("failed to initialize .nodejs junction: %w", err)
		}
	default:
		return fmt.Errorf("unsupported operating mode %q during profile initialization", mode)
	}

	if err := writeBootstrapVersion(currentBootstrapVersion); err != nil {
		return fmt.Errorf("failed to write bootstrap version marker: %w", err)
	}

	return nil
}

func profileNeedsRepair(version uint32, mode, shimDir, linkDir, nodejsPath, dataProxyPath, activeVersion string) (bool, error) {
	if version < currentBootstrapVersion {
		return true, nil
	}

	requiredPaths := []string{shimDir, linkDir, nodejsPath, dataProxyPath}
	if mode == "link" && strings.TrimSpace(activeVersion) != "" {
		linkNodePath, err := LinkNodePath()
		if err != nil {
			return false, err
		}
		requiredPaths = append(requiredPaths, linkNodePath)
	}

	for _, candidate := range requiredPaths {
		if _, err := os.Lstat(candidate); err != nil {
			if os.IsNotExist(err) {
				return true, nil
			}
			return false, fmt.Errorf("failed to inspect runtime path %s: %w", candidate, err)
		}
	}

	return false, nil
}

type bootstrapState struct {
	version          uint32
	hasVersionMarker bool
	hasLegacyMarker  bool
}

func (s bootstrapState) needsMarkerUpgrade() bool {
	return s.version == currentBootstrapVersion && (!s.hasVersionMarker || s.hasLegacyMarker)
}

func currentBootstrapState() (bootstrapState, error) {
	value, exists, err := registry.Get(bootstrapVersionPath())
	if err != nil {
		return bootstrapState{}, err
	}
	if exists {
		version, err := normalizeBootstrapVersion(value)
		if err != nil {
			return bootstrapState{}, err
		}
		return bootstrapState{version: version, hasVersionMarker: true}, nil
	}

	initialized, legacyExists, err := registry.GetBool(legacyInitializationMarkerPath())
	if err != nil {
		return bootstrapState{}, err
	}
	if legacyExists && initialized {
		return bootstrapState{version: bootstrapVersionV1, hasLegacyMarker: true}, nil
	}

	return bootstrapState{}, nil
}

func normalizeBootstrapVersion(value interface{}) (uint32, error) {
	switch v := value.(type) {
	case uint32:
		return v, nil
	case uint64:
		return uint32(v), nil
	case int:
		return uint32(v), nil
	case int32:
		return uint32(v), nil
	case int64:
		return uint32(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, nil
		}
		var parsed uint32
		if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
			return 0, fmt.Errorf("invalid bootstrap version %q", v)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported bootstrap version type %T", value)
	}
}

func writeBootstrapVersion(version uint32) error {
	if err := registry.Put(version, bootstrapVersionPath()); err != nil {
		return err
	}

	if err := registry.Del(legacyInitializationMarkerPath()); err != nil {
		return err
	}

	return nil
}

func bootstrapVersionPath() string {
	return prefs.ROOT + "/" + bootstrapVersionValueName
}

func legacyInitializationMarkerPath() string {
	return prefs.ROOT + "/" + legacyUserProfileInitializedValueName
}
