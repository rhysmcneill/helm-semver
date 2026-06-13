package chart

import (
	"os"
	"path/filepath"
	"testing"
)

func writeChart(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Chart.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp chart: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Metadata
		wantErr bool
	}{
		{
			name: "valid chart",
			content: `apiVersion: v2
name: observability
version: 0.1.0
appVersion: "1.0.0"
`,
			want: &Metadata{Name: "observability", Version: "0.1.0", AppVersion: "1.0.0"},
		},
		{
			name: "missing name",
			content: `apiVersion: v2
version: 0.1.0
`,
			wantErr: true,
		},
		{
			name: "missing version",
			content: `apiVersion: v2
name: myapp
`,
			wantErr: true,
		},
		{
			name:    "malformed yaml",
			content: `name: [unclosed`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeChart(t, tt.content)
			got, err := Load(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				if got.Name != tt.want.Name {
					t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
				}
				if got.Version != tt.want.Version {
					t.Errorf("Version = %q, want %q", got.Version, tt.want.Version)
				}
			}
		})
	}
}

func TestBumpVersion(t *testing.T) {
	original := `apiVersion: v2
name: observability
# this comment must be preserved
version: 0.1.0
appVersion: "1.0.0"
description: Umbrella chart
`
	path := writeChart(t, original)

	if err := BumpVersion(path, "0.2.0"); err != nil {
		t.Fatalf("BumpVersion() error = %v", err)
	}

	// Reload and verify version changed.
	m, err := Load(path)
	if err != nil {
		t.Fatalf("Load() after bump error = %v", err)
	}
	if m.Version != "0.2.0" {
		t.Errorf("Version after bump = %q, want %q", m.Version, "0.2.0")
	}
	// appVersion must not change.
	if m.AppVersion != "1.0.0" {
		t.Errorf("AppVersion changed unexpectedly: got %q", m.AppVersion)
	}

	// Raw content should still contain the comment.
	raw, _ := os.ReadFile(path)
	if !containsString(string(raw), "# this comment must be preserved") {
		t.Error("BumpVersion removed comment from Chart.yaml")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/Chart.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
