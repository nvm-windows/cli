package installer

import (
	nvmhttp "common/http"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const unauthorizedMirrorMessage = "You are not authorized to download from this server/mirror."

type mirrorAccessError struct {
	message string
}

func (e *mirrorAccessError) Error() string {
	return e.message
}

func isMirrorAccessError(err error) bool {
	var accessErr *mirrorAccessError
	return errors.As(err, &accessErr)
}

// mirrorAuthError returns a user-facing error for single-mirror 401/403 responses.
// Returns nil when the result is not an auth/policy denial (or response is missing).
func mirrorAuthError(result nvmhttp.DownloadResult, singleMirror bool) error {
	if !singleMirror || result.Response == nil || result.Response.Response == nil {
		return nil
	}

	statusCode := result.Response.Response.StatusCode
	switch statusCode {
	case http.StatusUnauthorized:
		return &mirrorAccessError{message: unauthorizedMirrorMessage}
	case http.StatusForbidden:
		message := strings.TrimSpace(string(result.Response.Content))
		if message == "" {
			message = strings.TrimSpace(result.Response.Response.Status)
		}
		if message == "" {
			message = "Forbidden"
		}
		if statusCodePrefix := fmt.Sprintf("%d ", statusCode); strings.HasPrefix(message, statusCodePrefix) {
			message = strings.TrimSpace(strings.TrimPrefix(message, statusCodePrefix))
		}
		return &mirrorAccessError{message: message}
	default:
		return nil
	}
}
