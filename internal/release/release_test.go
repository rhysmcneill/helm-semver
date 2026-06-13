package release

import (
	"strings"
	"testing"
)

func TestBuildReleaseNotes(t *testing.T) {
	commits := []string{
		"feat: add OCI support",
		"fix: correct tag prefix",
		"chore: update deps",
	}

	notes := BuildReleaseNotes(commits)

	for _, c := range commits {
		if !strings.Contains(notes, c) {
			t.Errorf("release notes missing commit %q:\n%s", c, notes)
		}
	}
	if !strings.HasPrefix(notes, "- ") {
		t.Errorf("expected notes to start with '- ', got: %s", notes)
	}
}

func TestBuildReleaseNotes_Empty(t *testing.T) {
	notes := BuildReleaseNotes(nil)
	if notes != "" {
		t.Errorf("expected empty string for nil commits, got %q", notes)
	}
}
