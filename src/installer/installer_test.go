package installer

import (
	"os"
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_EXPAND_VAR", "expanded_value")
	defer os.Unsetenv("TEST_EXPAND_VAR")

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "Expand single variable",
			input:    "%TEST_EXPAND_VAR%",
			contains: "expanded_value",
		},
		{
			name:     "Expand variable in path",
			input:    "%TEST_EXPAND_VAR%\\subfolder",
			contains: "expanded_value\\subfolder",
		},
		{
			name:     "Unknown variable remains unchanged",
			input:    "%UNKNOWN_VAR%\\path",
			contains: "%UNKNOWN_VAR%\\path",
		},
		{
			name:     "No variables",
			input:    "C:\\Users\\test",
			contains: "C:\\Users\\test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expand(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expand() = %q, want it to contain %q", result, tt.contains)
			}
		})
	}
}

func TestExpandRealVars(t *testing.T) {
	// Test with actual system variables
	tests := []struct {
		name  string
		input string
	}{
		{name: "APPDATA variable", input: "%APPDATA%"},
		{name: "LOCALAPPDATA variable", input: "%LOCALAPPDATA%"},
		{name: "USERPROFILE variable", input: "%USERPROFILE%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expand(tt.input)
			// Result should either be expanded (not contain %) or unchanged if var doesn't exist
			if strings.Contains(result, "%") {
				// It's OK if the variable doesn't exist, it should remain as-is
				if result != tt.input {
					t.Errorf("expand() = %q, expected either %q or expanded value", result, tt.input)
				}
			}
		})
	}
}
