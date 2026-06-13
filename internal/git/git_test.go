package git

import (
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initTestRepo creates a temporary git repo with an initial commit and returns
// the Client and the repo root path.
func initTestRepo(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	wt, _ := repo.Worktree()

	// Seed with an initial file and commit.
	seedFile := filepath.Join(dir, "README.md")
	_ = os.WriteFile(seedFile, []byte("# test"), 0o644)
	_, _ = wt.Add("README.md")
	_, err = wt.Commit("chore: initial commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	})
	if err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	return &Client{repo: repo}, dir
}

func addCommit(t *testing.T, c *Client, dir, file, msg string) {
	t.Helper()
	wt, _ := c.repo.Worktree()
	fullPath := filepath.Join(dir, file)
	_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
	_ = os.WriteFile(fullPath, []byte(msg), 0o644)
	_, _ = wt.Add(file)
	_, err := wt.Commit(msg, &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	})
	if err != nil {
		t.Fatalf("commit %q: %v", msg, err)
	}
}

func TestLatestTag_NoTags(t *testing.T) {
	c, _ := initTestRepo(t)
	tag, err := c.LatestTag("myapp")
	if err != nil {
		t.Fatalf("LatestTag() error = %v", err)
	}
	if tag != "" {
		t.Errorf("LatestTag() = %q, want empty", tag)
	}
}

func TestLatestTag_ReturnsLatest(t *testing.T) {
	c, dir := initTestRepo(t)
	addCommit(t, c, dir, "charts/myapp/Chart.yaml", "fix: patch 1")

	if err := c.Tag("myapp-v0.1.0"); err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	addCommit(t, c, dir, "charts/myapp/Chart.yaml", "feat: minor 1")
	if err := c.Tag("myapp-v0.2.0"); err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	tag, err := c.LatestTag("myapp")
	if err != nil {
		t.Fatalf("LatestTag() error = %v", err)
	}
	if tag != "myapp-v0.2.0" {
		t.Errorf("LatestTag() = %q, want %q", tag, "myapp-v0.2.0")
	}
}

func TestCommitsSince_NoTag(t *testing.T) {
	c, dir := initTestRepo(t)
	addCommit(t, c, dir, "charts/app/values.yaml", "feat: add redis")
	addCommit(t, c, dir, "charts/app/Chart.yaml", "fix: bump version")

	commits, err := c.CommitsSince("", "charts/app")
	if err != nil {
		t.Fatalf("CommitsSince() error = %v", err)
	}
	if len(commits) < 2 {
		t.Errorf("CommitsSince() returned %d commits, want >= 2", len(commits))
	}
}

func TestStageAndCommit(t *testing.T) {
	c, dir := initTestRepo(t)

	newFile := filepath.Join(dir, "charts/app/Chart.yaml")
	_ = os.MkdirAll(filepath.Dir(newFile), 0o755)
	_ = os.WriteFile(newFile, []byte("version: 0.2.0"), 0o644)

	if err := c.StageFile("charts/app/Chart.yaml"); err != nil {
		t.Fatalf("StageFile() error = %v", err)
	}

	if err := c.Commit("chore(app): release v0.2.0 [skip ci]", "bot", "bot@bot.com"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	head, _ := c.repo.Head()
	commit, _ := c.repo.CommitObject(head.Hash())
	if commit.Message != "chore(app): release v0.2.0 [skip ci]" {
		t.Errorf("commit message = %q", commit.Message)
	}
}
