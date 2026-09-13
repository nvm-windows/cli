package installer

import (
	nvmhttp "common/http"
	"fmt"
	"net/http"
	"strings"
)

func describeDownloadResultFailure(label, url string, result nvmhttp.DownloadResult) error {
	if result.Error != nil {
		return fmt.Errorf("%s %s: %w", label, url, result.Error)
	}
	if result.Response == nil || result.Response.Response == nil {
		return fmt.Errorf("%s %s: empty response", label, url)
	}
	status := result.Response.Response.StatusCode
	statusText := strings.TrimSpace(result.Response.Response.Status)
	if statusText == "" {
		statusText = http.StatusText(status)
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("%s %s: HTTP %s (missing on this mirror)", label, url, statusText)
	}
	return fmt.Errorf("%s %s: HTTP %s", label, url, statusText)
}

func formatNodeMirrorDownloadFailure(version, archiveName string, mirrors []string, lastErr error) error {
	mirrorNote := "configured mirror"
	if len(mirrors) > 1 {
		mirrorNote = fmt.Sprintf("%d configured mirrors", len(mirrors))
	}
	if lastErr == nil {
		return fmt.Errorf(
			"failed to download Node.js v%s (%s) from %s",
			version,
			archiveName,
			mirrorNote,
		)
	}
	return fmt.Errorf(
		"failed to download Node.js v%s (%s) from %s: %w",
		version,
		archiveName,
		mirrorNote,
		lastErr,
	)
}
