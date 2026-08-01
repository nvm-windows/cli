package installer

import (
	"testing"
)

func TestPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	child := root + `\node-v22.0.0-win-x64.7z`

	tests := []struct {
		name  string
		path  string
		root  string
		want  bool
	}{
		{name: "child file", path: child, root: root, want: true},
		{name: "root itself", path: root, root: root, want: true},
		{name: "outside sibling", path: t.TempDir() + `\other.7z`, root: root, want: false},
		{name: "empty path", path: "", root: root, want: false},
		{name: "empty root", path: child, root: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pathWithinRoot(tc.path, tc.root)
			if err != nil {
				t.Fatalf("pathWithinRoot() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("pathWithinRoot() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLocalSHASUMPath(t *testing.T) {
	got := localSHASUMPath("22.0.0", `C:\cache\node-v22.0.0-win-x64.7z`)
	want := `C:\cache\SHASUMS256-v22.0.0-win-x64.txt`
	if got != want {
		t.Fatalf("localSHASUMPath() = %q, want %q", got, want)
	}
}
