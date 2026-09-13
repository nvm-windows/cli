package installer

import (
	nvmhttp "common/http"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestDescribeDownloadResultFailureTransport(t *testing.T) {
	err := describeDownloadResultFailure("archive", "https://example/a.7z", nvmhttp.DownloadResult{
		Error: errors.New("context deadline exceeded"),
	})
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "archive https://example/a.7z") {
		t.Fatalf("error = %v, want label+url", err)
	}
}

func TestDescribeDownloadResultFailureHTTPStatus(t *testing.T) {
	res := &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found"}
	err := describeDownloadResultFailure("checksum", "https://example/SHASUMS256.txt", nvmhttp.DownloadResult{
		Response: &nvmhttp.DownloadResponse{Response: res},
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404 Not Found") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "missing on this mirror") {
		t.Fatalf("error = %v, want missing hint", err)
	}
}

func TestFormatNodeMirrorDownloadFailureWrapsCause(t *testing.T) {
	err := formatNodeMirrorDownloadFailure("24.20.0", "node-v24.20.0-win-x64.7z", []string{"https://nodejs.org/dist"}, errors.New("timeout"))
	got := err.Error()
	for _, want := range []string{
		"failed to download Node.js v24.20.0",
		"node-v24.20.0-win-x64.7z",
		"configured mirror",
		"timeout",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want substring %q", got, want)
		}
	}
	if strings.Contains(got, "not found on server/mirror") {
		t.Fatalf("legacy misleading message still present: %q", got)
	}
}

func TestFormatNodeMirrorDownloadFailurePluralMirrors(t *testing.T) {
	err := formatNodeMirrorDownloadFailure("22.0.0", "node-v22.0.0-win-x64.7z", []string{"a", "b"}, nil)
	if !strings.Contains(err.Error(), "2 configured mirrors") {
		t.Fatalf("error = %q", err)
	}
}
