package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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
	_ = os.WriteFile(seedFile, []byte("# test"), 0o600)
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
	_ = os.MkdirAll(filepath.Dir(fullPath), 0o750)
	_ = os.WriteFile(fullPath, []byte(msg), 0o600)
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
	_ = os.MkdirAll(filepath.Dir(newFile), 0o750)
	_ = os.WriteFile(newFile, []byte("version: 0.2.0"), 0o600)

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

func TestCommit_TimestampIsRecent(t *testing.T) {
	c, dir := initTestRepo(t)

	newFile := filepath.Join(dir, "foo.txt")
	_ = os.WriteFile(newFile, []byte("x"), 0o600)
	_ = c.StageFile("foo.txt")

	before := time.Now().Add(-time.Second)
	if err := c.Commit("test: check timestamp", "bot", "bot@example.com"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	after := time.Now().Add(time.Second)

	head, _ := c.repo.Head()
	commit, _ := c.repo.CommitObject(head.Hash())

	if commit.Author.When.Before(before) || commit.Author.When.After(after) {
		t.Errorf("commit timestamp %v outside expected range [%v, %v]",
			commit.Author.When, before, after)
	}
}

func TestRemoteIsHTTPS(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"https", "https://github.com/user/repo.git", true},
		{"http", "http://localhost/repo.git", true},
		{"ssh scp-style", "git@github.com:user/repo.git", false},
		{"ssh url", "ssh://git@github.com/user/repo.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := initTestRepo(t)
			_, err := c.repo.CreateRemote(&config.RemoteConfig{
				Name: "testremote",
				URLs: []string{tt.url},
			})
			if err != nil {
				t.Fatalf("create remote: %v", err)
			}
			got := c.remoteIsHTTPS("testremote")
			if got != tt.want {
				t.Errorf("remoteIsHTTPS(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestRemoteIsHTTPS_UnknownRemote(t *testing.T) {
	c, _ := initTestRepo(t)
	// Non-existent remote should default to true (assume HTTPS).
	if !c.remoteIsHTTPS("nonexistent") {
		t.Error("remoteIsHTTPS(nonexistent) = false, want true")
	}
}

func TestPush_LocalBareRemote(t *testing.T) {
	c, dir := initTestRepo(t)
	addCommit(t, c, dir, "charts/app/Chart.yaml", "feat: something")

	bareDir := t.TempDir()
	if _, err := gogit.PlainInit(bareDir, true); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}

	if _, err := c.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareDir},
	}); err != nil {
		t.Fatalf("create remote: %v", err)
	}

	// Push with no token to a local file remote — should succeed without auth.
	if err := c.Push("origin", ""); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
}
