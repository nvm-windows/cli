package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero", bytes: 0, want: "0 MB"},
		{name: "megabytes", bytes: 113 * 1024 * 1024, want: "113 MB"},
		{name: "exact one gib", bytes: 1024 * 1024 * 1024, want: "1.0 GB"},
		{name: "fractional gib", bytes: 3328599654, want: "3.1 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Fatalf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestGlobalModuleStatsTotalsAndUnique(t *testing.T) {
	root := t.TempDir()

	versions := []string{"v18.20.8", "v20.20.2", "v22.22.2"}
	for _, version := range versions {
		nodeModules := filepath.Join(root, version, "node_modules")
		if err := os.MkdirAll(nodeModules, 0755); err != nil {
			t.Fatalf("failed to create node_modules for %s: %v", version, err)
		}

		for _, module := range []string{"npm", "corepack"} {
			modulePath := filepath.Join(nodeModules, module)
			if err := os.MkdirAll(modulePath, 0755); err != nil {
				t.Fatalf("failed to create module %s for %s: %v", module, version, err)
			}
		}
	}

	total, unique, size := globalModuleStats(root)
	if total != 6 {
		t.Fatalf("total count = %d, want 6", total)
	}
	if unique != 2 {
		t.Fatalf("unique count = %d, want 2", unique)
	}
	if size < 0 {
		t.Fatalf("size = %d, want >= 0", size)
	}
}
