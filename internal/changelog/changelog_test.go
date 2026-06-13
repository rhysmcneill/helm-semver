package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var fixedDate = time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)

func TestAppend_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	commits := []string{
		"feat: add OCI support",
		"fix: correct tag prefix",
		"chore: update deps",
		"ci: add coverage check",
		"docs: update readme",
	}

	if err := Append(path, "0.2.0", fixedDate, commits); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "## [0.2.0] - 2026-06-13") {
		t.Errorf("missing version header in:\n%s", content)
	}
	if !strings.Contains(content, "### Features") {
		t.Errorf("missing Features section in:\n%s", content)
	}
	if !strings.Contains(content, "### Fixed") {
		t.Errorf("missing Fixed section in:\n%s", content)
	}
	// chore/ci/docs must not appear in the changelog
	if strings.Contains(content, "update deps") {
		t.Errorf("chore commit should not appear in changelog:\n%s", content)
	}
	if strings.Contains(content, "add coverage check") {
		t.Errorf("ci commit should not appear in changelog:\n%s", content)
	}
}

func TestAppend_Prepends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	_ = Append(path, "0.1.0", fixedDate, []string{"fix: initial release"})
	_ = Append(path, "0.2.0", fixedDate.AddDate(0, 0, 1), []string{"feat: new flag"})

	data, _ := os.ReadFile(path)
	content := string(data)

	idx02 := strings.Index(content, "0.2.0")
	idx01 := strings.Index(content, "0.1.0")

	if idx02 > idx01 {
		t.Errorf("expected 0.2.0 to appear before 0.1.0 in changelog")
	}
}

func TestAppend_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	existing := "# Changelog\n\n## [0.1.0] - 2026-01-01\n\n### Fixed\n- fix: old entry\n"
	_ = os.WriteFile(path, []byte(existing), 0o644)

	_ = Append(path, "0.2.0", fixedDate, []string{"feat: new feature"})

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "old entry") {
		t.Errorf("existing content was lost:\n%s", content)
	}
	if !strings.Contains(content, "0.2.0") {
		t.Errorf("new version missing:\n%s", content)
	}
}
