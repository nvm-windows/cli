package installer

import (
	nvmhttp "common/http"
	"net/http"
	"testing"
)

func TestMirrorAuthErrorUnauthorized(t *testing.T) {
	result := nvmhttp.DownloadResult{
		Response: &nvmhttp.DownloadResponse{
			Response: &http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized"},
			Content:  []byte("Unauthorized"),
		},
	}

	err := mirrorAuthError(result, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != unauthorizedMirrorMessage {
		t.Fatalf("error = %q, want %q", err.Error(), unauthorizedMirrorMessage)
	}
	if !isMirrorAccessError(err) {
		t.Fatal("expected mirror access error type")
	}
}

func TestMirrorAuthErrorForbiddenUsesBody(t *testing.T) {
	result := nvmhttp.DownloadResult{
		Response: &nvmhttp.DownloadResponse{
			Response: &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden"},
			Content:  []byte("blocked by policy"),
		},
	}

	err := mirrorAuthError(result, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "blocked by policy" {
		t.Fatalf("error = %q, want %q", err.Error(), "blocked by policy")
	}
}

func TestMirrorAuthErrorIgnoredForMultipleMirrors(t *testing.T) {
	result := nvmhttp.DownloadResult{
		Response: &nvmhttp.DownloadResponse{
			Response: &http.Response{StatusCode: http.StatusUnauthorized},
		},
	}
	if err := mirrorAuthError(result, false); err != nil {
		t.Fatalf("expected nil for multi-mirror, got %v", err)
	}
}

func TestMirrorAuthErrorIgnoredForOtherStatus(t *testing.T) {
	result := nvmhttp.DownloadResult{
		Response: &nvmhttp.DownloadResponse{
			Response: &http.Response{StatusCode: http.StatusNotFound},
			Content:  []byte("missing"),
		},
	}
	if err := mirrorAuthError(result, true); err != nil {
		t.Fatalf("expected nil for 404, got %v", err)
	}
}
