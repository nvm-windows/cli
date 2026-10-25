package commands

import (
	"fmt"
	neturl "net/url"
	"strings"
)

// HandleUpgradeProtocolURI runs a toast Apply/Install/Update action
// (nvm://upgrade?version=x.y.z). Force skips the process-close prompt
// because protocol activation has no console.
func HandleUpgradeProtocolURI(raw string) error {
	version, err := parseUpgradeProtocolVersion(raw)
	if err != nil {
		return err
	}
	return (&Upgrade{Force: true, Release: version}).Run()
}

func parseUpgradeProtocolVersion(raw string) (string, error) {
	raw = strings.Trim(strings.TrimSpace(raw), `"'`)
	u, err := neturl.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid nvm protocol URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "nvm") {
		return "", fmt.Errorf("unsupported protocol %q", u.Scheme)
	}
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if host == "" {
		host = strings.ToLower(strings.Trim(u.Path, "/"))
	}
	if host != "upgrade" {
		return "", fmt.Errorf("unsupported nvm:// host %q", host)
	}
	version := strings.TrimPrefix(strings.TrimSpace(u.Query().Get("version")), "v")
	if version == "" {
		return "", fmt.Errorf("upgrade requires version")
	}
	return version, nil
}
