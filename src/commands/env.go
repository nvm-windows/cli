package commands

import (
	nvmhttp "common/http"
	"common/inspect"
	"common/license"
	"common/registry"
	"common/settings"
	"common/system"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"nvm/bootstrap"
	"nvm/commands/cache"
	"nvm/constant"
	"nvm/status"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
)

var (
	helpURL string = "https://docs.nvm-windows.com"
)

const remoteReachabilityTimeout = 1500 * time.Millisecond

// Use common/http so Author mirrors (mirror.author.io) get Bearer access token.
var reachabilityClient = nvmhttp.NewClient(remoteReachabilityTimeout)

type Env struct {
	constant.FlagJSON
}

type installData struct {
	Version    string            `json:"version"`
	BuildTime  string            `json:"build_time"`
	InstallDir string            `json:"path"`
	Upgrade    string            `json:"upgrade"`
	Variables  map[string]string `json:"variables"`
}

type vmOps struct {
	Status                string          `json:"status"`
	Mode                  string          `json:"mode"`
	NodeMirror            []string        `json:"mirror_nodejs"`
	NodeMirrorPingResult  map[string]bool `json:"mirror_nodejs_ping_reachable"`
	NpmMirror             []string        `json:"mirror_npm"`
	NpmMirrorPingResult   map[string]bool `json:"mirror_npm_ping_reachable"`
	Root                  string          `json:"version_root"`
	ActiveVersion         string          `json:"version_active"`
	InstalledVersionCount int             `json:"version_install_count"`
	InstallSizeMB         int64           `json:"version_install_size_mb"`
	NpmModuleSizeMB       int64           `json:"npm_global_module_size_mb"`
	NpmGlobalModuleTotal  int64           `json:"npm_global_module_total_count"`
	NpmGlobalModuleUnique int64           `json:"npm_global_module_unique_count"`
	VersionsCache         string          `json:"cache_runtime"`
	VersionsCacheRoot     string          `json:"cache_root"`
	CachedVersionsCount   int             `json:"cache_count"`
	CachedVersionsSizeMB  int64           `json:"cache_size_mb"`
}

type Computer struct {
	MajorLabel      string   `json:"windows_major_label"`
	MajorVersion    int64    `json:"windows_major_version"`
	MajorVersionInt int64    `json:"windows_major_version_int"`
	MinorVersion    int64    `json:"windows_minor_version"`
	BuildNumber     string   `json:"windows_build_number"`
	UBR             int64    `json:"windows_build_revision"`
	DisplayVersion  string   `json:"windows_display_version"`
	InstallType     string   `json:"windows_installation_type"`
	Shell           string   `json:"current_shell"`
	DeveloperMode   bool     `json:"developer_mode_enabled"`
	Administrator   bool     `json:"user_is_admin"`
	Domain          string   `json:"domain"`
	Username        string   `json:"username"`
	UserID          string   `json:"user_id"`
	IPv6Enabled     bool     `json:"ipv6_enabled"`
	IPv6Interfaces  []string `json:"ipv6_interfaces"`
}

type data struct {
	Installation      installData `json:"installation"`
	VersionManagement vmOps       `json:"operations"`
	Computer          Computer    `json:"localhost"`
	ActiveLicense     *License    `json:"license,omitempty"`
	ReportStatus      string      `json:"report_status,omitempty"`
	Help              string      `json:"help_url,omitempty"`
}

type License struct {
	Plan         string   `json:"plan"`
	Roles        []string `json:"roles"`
	Issued       string   `json:"issued"`
	Expires      string   `json:"expires"`
	Verification string   `json:"verification,omitempty"`
}

var (
	line   string = "│ "
	branch string = "├─"
	end    string = "└─"
	br     string = "\t\n"
)

func (e *Env) Run(ctx *kong.Context, vars kong.Vars) error {
	var spinner *status.Spinner
	if !e.JSON {
		spinner = status.NewSpinner("Analyzing environment")
		defer spinner.Stop()
		spinner.Start()
	}

	start := time.Now()
	cfg := settings.Global()
	programRoot, err := bootstrap.ProgramRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve program root: %w", err)
	}
	installRoot := settings.Expand(cfg.Root)
	cachedRuntimeCount, cachedRuntimeSizeBytes := cacheStats(cache.Store.Versions)
	_, installSizeBytes := cacheStats(installRoot)
	installCount := installedVersionCount(installRoot)
	moduleTotalCount, moduleUniqueCount, moduleSizeBytes := globalModuleStats(installRoot)

	win_major_version, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/CurrentMajorVersionNumber")
	win_minor_version, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/CurrentMinorVersionNumber")
	win_build_number, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/CurrentBuildNumber")
	win_build_revision, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/UBR")
	win_display_version, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/DisplayVersion")
	win_installation_type, _, _ := registry.Get("HKLM/SOFTWARE/Microsoft/Windows NT/CurrentVersion/InstallationType")
	win_product_type, _, _ := registry.Get("HKLM/SYSTEM/CurrentControlSet/Control/ProductOptions/ProductType")

	win_major_label := "10"
	build, _ := strconv.Atoi(win_build_number.(string))
	installationType := ""
	if v, ok := win_installation_type.(string); ok {
		installationType = strings.TrimSpace(v)
	}
	installationTypeLower := strings.ToLower(installationType)
	productType := ""
	if v, ok := win_product_type.(string); ok {
		productType = strings.TrimSpace(v)
	}

	isServer := strings.EqualFold(productType, "ServerNT") || strings.EqualFold(productType, "LanmanNT")

	if isServer {
		editionSuffix := ""
		if installationTypeLower == "server core" {
			editionSuffix = " Core"
		}

		switch {
		case build >= 26100:
			win_major_label = "Server 2025" + editionSuffix
		case build >= 20348:
			win_major_label = "Server 2022" + editionSuffix
		case build >= 17763:
			win_major_label = "Server 2019" + editionSuffix
		case build >= 14393:
			win_major_label = "Server 2016" + editionSuffix
		default:
			win_major_label = "Server" + editionSuffix
		}
	} else if win_major_version.(uint64) == uint64(10) && build >= 22000 {
		win_major_label = "11"
	} else {
		win_major_label = fmt.Sprintf("%d", win_major_version.(uint64))
	}

	ipv6enabled := false
	ipv6interfaces := []string{}
	shellEnv := getShellEnv()
	developerModeEnabled := isDeveloperModeEnabled()
	isAdministrator, _ := system.IsAdministrator()

	currentUser, _ := user.Current()
	userDomain := ""
	userName := ""
	userID := ""
	if currentUser != nil {
		parts := strings.SplitN(currentUser.Username, "\\", 2)
		if len(parts) == 2 {
			userDomain = parts[0]
			userName = parts[1]
		} else {
			userName = currentUser.Username
		}
		userID = currentUser.Uid
	}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		ipv6Candidates := []net.IP{}

		addrs, _ := iface.Addrs()

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() == nil { // It's IPv6
					ipv6Candidates = append(ipv6Candidates, ipnet.IP)
				}
			}
		}

		if isInternetIPv6Interface(iface, ipv6Candidates) {
			ipv6enabled = true
			ipv6interfaces = append(ipv6interfaces, iface.Name)
		}
	}

	if !ipv6enabled {
		ipv6, _, _ := registry.Get("HKLM/SYSTEM/CurrentControlSet/Services/Tcpip6/Parameters/DisabledComponents")
		if ipv6 == "1" {
			ipv6enabled = false
		} else {
			ipv6enabled = true
		}
	}

	node_ping_results := runReachabilityChecks(cfg.NodeMirror, isNodeMirrorReachable)
	npm_ping_results := runReachabilityChecks(cfg.NpmMirror, isNpmMirrorReachable)

	current_status := "on"
	if !cfg.Enabled {
		current_status = "off"
	}

	var activeLicense *License
	if info, ok := license.InspectAccessToken(); ok {
		activeLicense = &License{
			Plan:         info.Plan,
			Roles:        info.Roles,
			Issued:       info.Issued,
			Expires:      info.Expires,
			Verification: info.Verification,
		}
	}

	out := data{
		Installation: installData{
			Version:    vars["version"],
			BuildTime:  vars["buildTime"],
			InstallDir: path(programRoot),
			Upgrade:    map[bool]string{true: "blocked", false: "allowed"}[cfg.DisableUpgrade],
			// Variables: map[string]string{
			// 	"NVM_HOME":      getUserEnvVar("NVM_HOME"),
			// 	"NVM_NODE_PATH": getUserEnvVar("NVM_NODE_PATH"),
			// },
		},
		VersionManagement: vmOps{
			Status:                current_status,
			Mode:                  cfg.Mode,
			ActiveVersion:         cfg.ActiveVersion,
			NodeMirror:            cfg.NodeMirror,
			NodeMirrorPingResult:  node_ping_results,
			NpmMirror:             cfg.NpmMirror,
			NpmMirrorPingResult:   npm_ping_results,
			Root:                  path(cfg.Root),
			InstalledVersionCount: installCount,
			VersionsCacheRoot:     path(cache.Store.Versions),
			CachedVersionsCount:   cachedRuntimeCount,
			CachedVersionsSizeMB:  cachedRuntimeSizeBytes / (1024 * 1024),
			InstallSizeMB:         installSizeBytes / (1024 * 1024),
			NpmGlobalModuleTotal:  moduleTotalCount,
			NpmGlobalModuleUnique: moduleUniqueCount,
			NpmModuleSizeMB:       moduleSizeBytes / (1024 * 1024),
		},
		Computer: Computer{
			MajorLabel:     win_major_label,
			MajorVersion:   int64(win_major_version.(uint64)),
			MinorVersion:   int64(win_minor_version.(uint64)),
			BuildNumber:    win_build_number.(string),
			UBR:            int64(win_build_revision.(uint64)),
			DisplayVersion: win_display_version.(string),
			InstallType:    installationType,
			Shell:          shellEnv,
			DeveloperMode:  developerModeEnabled,
			Administrator:  isAdministrator,
			Domain:         userDomain,
			Username:       userName,
			UserID:         userID,
			IPv6Enabled:    ipv6enabled,
			IPv6Interfaces: ipv6interfaces,
		},
		ActiveLicense: activeLicense,
		Help:          helpURL,
		ReportStatus:  fmt.Sprintf("Completed in %s", time.Since(start)),
	}

	// JSON output
	if e.JSON {
		raw, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(raw))

		return nil
	} else {
		spinner.Stop()
	}

	t := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	defer func() {
		t.Flush()

		fmt.Fprintf(os.Stdout, "\n%s\n", out.ReportStatus)
		fmt.Fprintf(os.Stdout, "\nRun `nvm doctor` or visit %s for help.\n", out.Help)
	}()

	// OS Report
	fmt.Fprintf(t, "My Computer\t\n")
	fmt.Fprintf(t, "%s%s Edition\t: Windows %s %s (%d.%d.%s.%d)\n", indent(1), branch, out.Computer.MajorLabel, out.Computer.DisplayVersion, out.Computer.MajorVersion, out.Computer.MinorVersion, out.Computer.BuildNumber, out.Computer.UBR)
	fmt.Fprintf(t, "%s%s Domain/Machine\t: %s\n", indent(1), branch, out.Computer.Domain)
	fmt.Fprintf(t, "%s%s Username\t: %s\n", indent(1), branch, out.Computer.Username)
	fmt.Fprintf(t, "%s%s User ID\t: %s\n", indent(1), branch, out.Computer.UserID)
	fmt.Fprintf(t, "%s%s Developer Mode\t: %s\n", indent(1), branch, map[bool]string{true: "Enabled", false: "Disabled"}[out.Computer.DeveloperMode])
	fmt.Fprintf(t, "%s%s Administrator\t: %s\n", indent(1), branch, map[bool]string{true: "Yes", false: "No"}[out.Computer.Administrator])
	fmt.Fprintf(t, "%s%s Current Shell\t: %s\n", indent(1), branch, out.Computer.Shell)

	// IPv6
	for i, name := range ipv6interfaces {
		if i == 0 {
			fmt.Fprintf(t, "%s%s IPv6 Interfaces\t: %s\n", indent(1), end, name)
		} else {
			fmt.Fprintf(t, "%s     \t  %s\n", indent(2), name)
		}
	}

	if len(ipv6interfaces) == 0 {
		if ipv6enabled {
			fmt.Fprintf(t, "%s%s IPv6 Interfaces\t: Enabled\n", indent(1), branch)
		} else {
			fmt.Fprintf(t, "%s%s IPv6 Interfaces\t: Disabled\n", indent(1), branch)
		}
	}

	fmt.Fprint(t, br)

	// nvm Report
	fmt.Fprint(t, "NVM For Windows\t\n")
	// TODO: Available Updates

	// nvm version
	fmt.Fprintf(t, "%s%s Version\t: %s\n", indent(1), branch, out.Installation.Version)

	// nvm build
	fmt.Fprintf(t, "%s%s Build\t: %s\n", indent(1), branch, out.Installation.BuildTime)

	// nvm install root
	hasActiveLicense := out.ActiveLicense != nil

	if hasActiveLicense {
		free := " (free)"
		if !strings.EqualFold(out.ActiveLicense.Plan, "community") {
			free = ""
		}

		fmt.Fprintf(t, "%s%s License\t: %s%s\n", indent(1), branch, out.ActiveLicense.Plan, free)
		if out.ActiveLicense.Verification != "" {
			fmt.Fprintf(t, "%s%s Verification\t: %s\n", indent(1), branch, out.ActiveLicense.Verification)
		}
		fmt.Fprintf(t, "%s%s%s %s Issued\t: %s\n", indent(1), line, indent(1), branch, out.ActiveLicense.Issued)
		fmt.Fprintf(t, "%s%s%s %s Expires\t: %s\n", indent(1), line, indent(1), end, out.ActiveLicense.Expires)
	}

	// Environment Variables
	if len(out.Installation.Variables) > 0 {
		fmt.Fprintf(t, "%s%s Variables\t\n", indent(1), end)
		ct := 0
		for k, v := range out.Installation.Variables {
			ct++
			symbol := branch
			if ct == len(out.Installation.Variables) {
				symbol = end
			}

			fmt.Fprintf(t, "%s%s %s\t: %s\n", indent(7, " "), symbol, k, v)
		}
	}

	fmt.Fprintf(t, "%s%s Path\t: %s\n", indent(1), end, out.Installation.InstallDir)

	fmt.Fprint(t, br)

	// Version Management Report
	fmt.Fprint(t, "Version Management\t\n")

	// On/Off
	fmt.Fprintf(t, "%s%s Status\t: %s\n", indent(1), branch, out.VersionManagement.Status)

	// Operating Mode
	fmt.Fprintf(t, "%s%s Operating Mode\t: %s\n", indent(1), branch, out.VersionManagement.Mode)

	// Node Mirrors
	fmt.Fprintf(t, "%s%s Download Sources\t\n", indent(1), branch)
	for i, mirror := range out.VersionManagement.NodeMirror {
		reachable := ""
		if !out.VersionManagement.NodeMirrorPingResult[mirror] {
			reachable = " (unreachable)"
		}

		if i == 0 {
			fmt.Fprintf(t, "%s%s%s %s Node.js\t: %s%s\n", indent(1), line, indent(1), branch, mirror, reachable)
		} else {
			fmt.Fprintf(t, "%s%s%s %s        \t  %s%s\n", indent(1), line, indent(1), line, mirror, reachable)
		}
	}

	// npm Mirrors
	for i, mirror := range out.VersionManagement.NpmMirror {
		reachable := ""
		if !out.VersionManagement.NpmMirrorPingResult[mirror] {
			reachable = " (unreachable)"
		}

		if i == 0 {
			fmt.Fprintf(t, "%s%s%s %s npm\t: %s%s\n", indent(1), line, indent(1), end, mirror, reachable)
		} else {
			fmt.Fprintf(t, "%s%s%s %s      \t  %s%s\n", indent(1), line, indent(1), end, mirror, reachable)
		}
	}

	fmt.Fprintf(t, "%s%s Installed Versions\t\n", indent(1), branch)

	// Active Version
	activeVersion := "not set"
	if out.VersionManagement.ActiveVersion != "" {
		activeVersion = "v" + out.VersionManagement.ActiveVersion
	}
	fmt.Fprintf(t, "%s%s%s %s Default\t: %s\n", indent(1), line, indent(1), branch, activeVersion)

	// Installation Count
	fmt.Fprintf(t, "%s%s%s %s Total\t: %d (%s)\n", indent(1), line, indent(1), branch, out.VersionManagement.InstalledVersionCount, formatSize(installSizeBytes))

	fmt.Fprintf(t, "%s%s%s %s Modules\t: %d (%s) / %d unique\n", indent(1), line, indent(1), branch, out.VersionManagement.NpmGlobalModuleTotal, formatSize(moduleSizeBytes), out.VersionManagement.NpmGlobalModuleUnique)

	// Installation Root
	fmt.Fprintf(t, "%s%s%s %s Path\t: %s\n", indent(1), line, indent(1), end, path(out.VersionManagement.Root))

	// Footprint
	fmt.Fprintf(t, "%s%s Cache\t\n", indent(1), end)
	fmt.Fprintf(t, "%s%s Node.js Versions\t: %d\n", indent(7, " "), branch, out.VersionManagement.CachedVersionsCount)
	fmt.Fprintf(t, "%s%s Total Size\t: %s\n", indent(7, " "), branch, formatSize(cachedRuntimeSizeBytes))
	fmt.Fprintf(t, "%s%s Path\t: %s\n", indent(7, " "), end, out.VersionManagement.VersionsCacheRoot)

	// Identify EOL versions and those w%shich are supported by nvm

	// Announcements

	// Display general help link at bottom (like --help)

	// Checks
	// Assure nvm is reachable on the, reachable PATH
	// Assure the shim or link directory is on the PATH and prioritized over any
	// Identify unmanaged %sNode.js installations on the PATH and warn a, reachablebout potential conflicts
	// Check the terminal to d%setermine what is supported (Windows onl, reachabley)
	// Identify issues connecting to mirror servers
	// Identify potentially invalid node versions (missing node.exe, missing npm)

	return nil
}

func showDetail(t *tabwriter.Writer, problem *inspect.Problem) {
	content := strings.TrimSpace(fmt.Sprintf("%s \n%s %s", problem.Name, problem.Detail, problem.Help))
	// content := strings.TrimSpace(fmt.Sprintf("%s %s \n%s %s", end, problem.Name, problem.Detail, problem.Help))
	wrapped := wrapWords(content, 86)
	if len(wrapped) == 0 {
		wrapped = []string{""}
	}
	// fmt.Fprintf(t, "%s  \t  %s\n", indent(1), wrapped[0])
	// fmt.Fprintf(t, "%s%s  \t  %s\n", indent(1), line, wrapped[0])
	for i := 0; i < len(wrapped); i++ {
		fmt.Fprintf(t, "%s%s  \t  %s\n", indent(1), line, wrapped[i])
	}
}

func path(p string) string {
	return strings.ReplaceAll(filepath.ToSlash(p), "/", "\\")
}

func indent(s int, char ...string) string {
	c := "  "
	if len(char) > 0 {
		c = char[0]
	}
	return strings.Repeat(c, s)
}

func formatSize(bytes int64) string {
	const (
		mb = int64(1024 * 1024)
		gb = int64(1024 * 1024 * 1024)
	)

	if bytes >= gb {
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	}

	return fmt.Sprintf("%d MB", bytes/mb)
}

func wrapWords(text string, maxLen int) []string {
	if maxLen <= 0 {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n")
	out := make([]string, 0, len(paragraphs))

	for _, paragraph := range paragraphs {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}

		current := words[0]
		for i := 1; i < len(words); i++ {
			next := words[i]
			if len(current)+1+len(next) <= maxLen {
				current += " " + next
				continue
			}

			out = append(out, current)
			current = next
		}

		out = append(out, current)
	}

	return out
}

func cacheStats(root string) (count int, sizeBytes int64) {
	if root == "" {
		return 0, 0
	}

	if _, err := os.Stat(root); err != nil {
		return 0, 0
	}

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}

		count++
		sizeBytes += info.Size()
		return nil
	})

	return count, sizeBytes
}

func installedVersionCount(root string) int {
	if root == "" {
		return 0
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if strings.HasPrefix(name, "v") {
			count++
		}
	}

	return count
}

func globalModuleStats(root string) (totalCount int64, uniqueCount int64, sizeBytes int64) {
	if root == "" {
		return 0, 0, 0
	}

	versions, err := os.ReadDir(root)
	if err != nil {
		return 0, 0, 0
	}

	unique := make(map[string]struct{})

	for _, version := range versions {
		if !version.IsDir() {
			continue
		}

		versionName := strings.ToLower(version.Name())
		if !strings.HasPrefix(versionName, "v") {
			continue
		}

		nodeModules := filepath.Join(root, version.Name(), "node_modules")
		entries, readErr := os.ReadDir(nodeModules)
		if readErr != nil {
			continue
		}

		_, dirSize := cacheStats(nodeModules)
		sizeBytes += dirSize

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if strings.HasPrefix(name, "@") {
				scopedEntries, scopedErr := os.ReadDir(filepath.Join(nodeModules, name))
				if scopedErr != nil {
					continue
				}

				for _, scoped := range scopedEntries {
					if !scoped.IsDir() {
						continue
					}
					totalCount++
					unique[name+"/"+scoped.Name()] = struct{}{}
				}
				continue
			}

			totalCount++
			unique[name] = struct{}{}
		}
	}

	return totalCount, int64(len(unique)), sizeBytes
}

func isInternetIPv6Interface(iface net.Interface, ips []net.IP) bool {
	if iface.Flags&net.FlagUp == 0 {
		return false
	}

	if iface.Flags&net.FlagLoopback != 0 {
		return false
	}

	name := strings.ToLower(iface.Name)
	ignored := []string{
		"bluetooth",
		"vethernet",
		"loopback",
		"isatap",
		"teredo",
		"6to4",
		"npcap",
		"vmware",
		"virtualbox",
	}

	for _, marker := range ignored {
		if strings.Contains(name, marker) {
			return false
		}
	}

	// Report adapters where IPv6 is enabled/present so users can identify
	// interfaces that may affect nvm networking behavior, even if a global
	// internet-routable address is not currently assigned.
	return len(ips) > 0
}

func getShellEnv() string {
	if os.Getenv("CMDER_ROOT") != "" {
		return "Cmder"
	}
	if os.Getenv("ConEmuPID") != "" || os.Getenv("ConEmuDir") != "" {
		return "ConEmu"
	}
	if os.Getenv("NU_VERSION") != "" {
		return "Nushell"
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return "WSL (Unsupported)"
	}
	if os.Getenv("CYGWIN") != "" || strings.Contains(strings.ToLower(os.Getenv("OSTYPE")), "cygwin") {
		return "Cygwin (Unsupported)"
	}
	if msystem := strings.ToUpper(os.Getenv("MSYSTEM")); msystem != "" {
		switch {
		case strings.HasPrefix(msystem, "MINGW"):
			return "Git Bash/MinGW (Unsupported)"
		case strings.HasPrefix(msystem, "MSYS"):
			return "MSYS2 (Unsupported)"
		default:
			return "Unix Emulator (Unsupported)"
		}
	}
	if os.Getenv("PSModulePath") != "" {
		if os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL") != "" {
			return "PowerShell"
		}
		return "PowerShell"
	}
	if os.Getenv("CLINK_ID") != "" {
		return "CMD/Clink"
	}
	if os.Getenv("PROMPT") != "" {
		return "CMD"
	}
	if os.Getenv("TERM") != "" || os.Getenv("SHELL") != "" {
		return "Unix-like Shell (Unsupported)"
	}
	return "Unknown (Unsupported)"
}

func isDeveloperModeEnabled() bool {
	devMode, _, err := registry.Get("HKLM/SOFTWARE/Microsoft/Windows/CurrentVersion/AppModelUnlock/AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}

	return devMode == uint64(1)
}

func startSpinner(label string) func() {
	frames := []string{"|", "/", "-", "\\"}
	done := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(stopped)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		for {
			fmt.Fprintf(os.Stderr, "\r%s %s", label, frames[i%len(frames)])
			i++

			select {
			case <-done:
				clearWidth := len(label) + 4
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", clearWidth))
				return
			case <-ticker.C:
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func getUserEnvVar(name string) string {
	if v, exists, err := registry.Get("HKCU/Environment/" + name); err == nil && exists {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}

	return os.Getenv(name)
}

func runReachabilityChecks(mirrors []string, checker func(string) bool) map[string]bool {
	results := make(map[string]bool, len(mirrors))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, m := range mirrors {
		mirror := m
		wg.Add(1)
		go func() {
			defer wg.Done()
			reachable := checker(mirror)
			mu.Lock()
			results[mirror] = reachable
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results
}

func isNodeMirrorReachable(url string) bool {
	res, err := reachabilityClient.Head(url + "/index.tab")
	if err != nil {
		return false
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return false
	}

	return true
}

func isNpmMirrorReachable(url string) bool {
	res, err := reachabilityClient.Get(url + "/-/ping")
	if err != nil {
		return false
	}
	defer res.Body.Close()

	return (res.StatusCode == 200)
}

func formatUserLocalDateTime(ts time.Time) string {
	dateLayout := windowsDateToGoLayout("M/d/yyyy")
	timeLayout := windowsTimeToGoLayout("h:mm tt")

	if raw, exists, err := registry.Get("HKCU/Control Panel/International/sShortDate"); err == nil && exists {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			dateLayout = windowsDateToGoLayout(s)
		}
	}

	if raw, exists, err := registry.Get("HKCU/Control Panel/International/sShortTime"); err == nil && exists {
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			timeLayout = windowsTimeToGoLayout(s)
		}
	}

	// JWT numeric dates are UTC epoch seconds. Convert using Windows active
	// timezone bias so output reflects the user's configured local timezone.
	local := time.Unix(ts.Unix(), 0).UTC().Add(-time.Duration(getWindowsActiveTimeBiasMinutes()) * time.Minute)
	return local.Format(dateLayout + " " + timeLayout)
}

func getWindowsActiveTimeBiasMinutes() int {
	if raw, exists, err := registry.Get("HKLM/SYSTEM/CurrentControlSet/Control/TimeZoneInformation/ActiveTimeBias"); err == nil && exists {
		switch v := raw.(type) {
		case uint64:
			return int(v)
		case uint32:
			return int(v)
		case int64:
			return int(v)
		case int32:
			return int(v)
		case string:
			if parsed, parseErr := strconv.Atoi(strings.TrimSpace(v)); parseErr == nil {
				return parsed
			}
		}
	}

	// Fallback to runtime local zone offset when registry isn't available.
	_, offsetSeconds := time.Now().Zone()
	return -(offsetSeconds / 60)
}

func windowsDateToGoLayout(format string) string {
	layout := strings.TrimSpace(format)
	replacements := [][2]string{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"MMMM", "January"},
		{"MMM", "Jan"},
		{"MM", "01"},
		{"M", "1"},
		{"dd", "02"},
		{"d", "2"},
	}

	for _, r := range replacements {
		layout = strings.ReplaceAll(layout, r[0], r[1])
	}

	if layout == "" {
		return "1/2/2006"
	}

	return layout
}

func windowsTimeToGoLayout(format string) string {
	layout := strings.TrimSpace(format)
	replacements := [][2]string{
		{"HH", "15"},
		{"H", "15"},
		{"hh", "03"},
		{"h", "3"},
		{"mm", "04"},
		{"m", "4"},
		{"ss", "05"},
		{"s", "5"},
		{"tt", "PM"},
		{"t", "P"},
	}

	for _, r := range replacements {
		layout = strings.ReplaceAll(layout, r[0], r[1])
	}

	if layout == "" {
		return "3:04 PM"
	}

	return layout
}
