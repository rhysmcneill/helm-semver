package release

import (
	"strings"
	"testing"

	igit "github.com/rhysmcneill/helm-semver/internal/git"
)

func TestBuildReleaseNotes(t *testing.T) {
	commits := []igit.CommitInfo{
		{Subject: "feat: add OCI support", Hash: "a1b2c3d4e5f67890", PR: 0},   // pragma: allowlist secret
		{Subject: "fix: correct tag prefix", Hash: "b2c3d4e5f6789012", PR: 0}, // pragma: allowlist secret
		{Subject: "chore: update deps", Hash: "c3d4e5f678901234", PR: 0},      // pragma: allowlist secret
	}

	notes := BuildReleaseNotes(commits, "", "")

	for _, c := range commits {
		if !strings.Contains(notes, c.Subject) {
			t.Errorf("release notes missing commit %q:\n%s", c.Subject, notes)
		}
	}
	if !strings.HasPrefix(notes, "- ") {
		t.Errorf("expected notes to start with '- ', got: %s", notes)
	}
}

func TestBuildReleaseNotes_WithLinks(t *testing.T) {
	commits := []igit.CommitInfo{
		{Subject: "fix: auth error (#12)", Hash: "a1b2c3d4e5f6a7b8", PR: 12}, // pragma: allowlist secret
	}

	notes := BuildReleaseNotes(commits, "owner", "repo")

	if !strings.Contains(notes, "[#12](https://github.com/owner/repo/pull/12)") {
		t.Errorf("missing PR link in release notes:\n%s", notes)
	}
	if !strings.Contains(notes, "[a1b2c3d](https://github.com/owner/repo/commit/a1b2c3d4e5f6a7b8)") {
		t.Errorf("missing commit SHA link in release notes:\n%s", notes)
	}
}

func TestBuildReleaseNotes_Empty(t *testing.T) {
	notes := BuildReleaseNotes(nil, "", "")
	if notes != "" {
		t.Errorf("expected empty string for nil commits, got %q", notes)
	}
}
