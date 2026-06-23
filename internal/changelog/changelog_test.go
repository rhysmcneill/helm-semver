package changelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	igit "github.com/rhysmcneill/helm-semver/internal/git"
)

var fixedDate = time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)

// makeCommits builds a []igit.CommitInfo slice from bare subject strings with
// stub hash values, for tests that don't need real SHAs.
func makeCommits(subjects ...string) []igit.CommitInfo {
	commits := make([]igit.CommitInfo, len(subjects))
	for i, s := range subjects {
		commits[i] = igit.CommitInfo{Subject: s, Hash: ""}
	}
	return commits
}

func TestAppend_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	commits := makeCommits(
		"feat: add OCI support",
		"fix: correct tag prefix",
		"chore: update deps",
		"ci: add coverage check",
		"docs: update readme",
	)

	if err := Append(path, "0.2.0", fixedDate, commits, "", "", RepoInfo{}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
	content := string(data)

	if !strings.Contains(content, "## [0.2.0] - 2026-06-13") {
		t.Errorf("missing version header in:\n%s", content)
	}
	if !strings.Contains(content, "### Features") {
		t.Errorf("missing Features section in:\n%s", content)
	}
	if !strings.Contains(content, "### Bug Fixes") {
		t.Errorf("missing Bug Fixes section in:\n%s", content)
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

	_ = Append(path, "0.1.0", fixedDate, makeCommits("fix: initial release"), "", "", RepoInfo{})
	_ = Append(path, "0.2.0", fixedDate.AddDate(0, 0, 1), makeCommits("feat: new flag"), "", "", RepoInfo{})

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
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

	existing := "# Changelog\n\n## [0.1.0] - 2026-01-01\n\n### Bug Fixes\n- fix: old entry\n"
	_ = os.WriteFile(path, []byte(existing), 0o600)

	_ = Append(path, "0.2.0", fixedDate, makeCommits("feat: new feature"), "", "", RepoInfo{})

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
	content := string(data)

	if !strings.Contains(content, "old entry") {
		t.Errorf("existing content was lost:\n%s", content)
	}
	if !strings.Contains(content, "0.2.0") {
		t.Errorf("new version missing:\n%s", content)
	}
}

func TestAppend_WithLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	commits := []igit.CommitInfo{
		{Subject: "fix: auth error (#12)", Hash: "a1b2c3d4e5f6a7b8", PR: 12},   // pragma: allowlist secret
		{Subject: "feat: new feature (#15)", Hash: "b2c3d4e5f6a7b8c9", PR: 15}, // pragma: allowlist secret
	}

	repo := RepoInfo{Owner: "owner", Name: "repo"}
	if err := Append(path, "1.0.5", fixedDate, commits, "chart-v1.0.4", "chart-v1.0.5", repo); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
	content := string(data)

	// Release heading with compare link.
	if !strings.Contains(content, "https://github.com/owner/repo/compare/chart-v1.0.4...chart-v1.0.5") {
		t.Errorf("missing compare link in heading:\n%s", content)
	}
	// PR hyperlink.
	if !strings.Contains(content, "[#12](https://github.com/owner/repo/pull/12)") {
		t.Errorf("missing PR link:\n%s", content)
	}
	// Commit SHA hyperlink (7-char short SHA).
	if !strings.Contains(content, "[a1b2c3d](https://github.com/owner/repo/commit/a1b2c3d4e5f6a7b8)") {
		t.Errorf("missing commit SHA link:\n%s", content)
	}
	if !strings.Contains(content, "### Bug Fixes") {
		t.Errorf("missing Bug Fixes section:\n%s", content)
	}
	if !strings.Contains(content, "### Features") {
		t.Errorf("missing Features section:\n%s", content)
	}
}

func TestAppend_BreakingChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	commits := []igit.CommitInfo{
		{Subject: "feat!: drop support for v1 (#20)", Hash: "deadbeef12345678", PR: 20}, // pragma: allowlist secret
	}

	if err := Append(path, "2.0.0", fixedDate, commits, "chart-v1.0.0", "chart-v2.0.0", RepoInfo{}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
	content := string(data)

	if !strings.Contains(content, "### ⚠ BREAKING CHANGES") {
		t.Errorf("missing breaking changes section heading:\n%s", content)
	}
}

func TestAppend_NoLinksWhenRepoEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CHANGELOG.md")

	commits := []igit.CommitInfo{
		{Subject: "fix: something (#5)", Hash: "abc123def456", PR: 5}, // pragma: allowlist secret
	}

	if err := Append(path, "1.0.1", fixedDate, commits, "chart-v1.0.0", "chart-v1.0.1", RepoInfo{}); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	data, _ := os.ReadFile(path) //nolint:gosec // path from t.TempDir()
	content := string(data)

	if strings.Contains(content, "github.com") {
		t.Errorf("links should not appear when RepoInfo is empty:\n%s", content)
	}
	// Plain version heading without compare URL.
	if !strings.Contains(content, "## [1.0.1] - 2026-06-13") {
		t.Errorf("missing plain version header:\n%s", content)
	}
}
