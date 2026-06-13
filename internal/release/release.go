// Package release creates GitHub Releases via the GitHub API.
package release

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API for creating releases.
type Client struct {
	gh    *github.Client
	owner string
	repo  string
}

// New constructs a Client from a GitHub token, owner, and repository name.
func New(token, owner, repo string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh:    github.NewClient(tc),
		owner: owner,
		repo:  repo,
	}
}

// CreateRelease creates a GitHub Release for the given tag and returns its URL.
func (c *Client) CreateRelease(ctx context.Context, tag, name, body string) (string, error) {
	rel, _, err := c.gh.Repositories.CreateRelease(ctx, c.owner, c.repo, &github.RepositoryRelease{
		TagName: github.String(tag),
		Name:    github.String(name),
		Body:    github.String(body),
	})
	if err != nil {
		return "", fmt.Errorf("creating GitHub release %s: %w", tag, err)
	}
	return rel.GetHTMLURL(), nil
}

// UploadAsset uploads a file as a release asset.
func (c *Client) UploadAsset(ctx context.Context, releaseID int64, assetPath string) error {
	f, err := os.Open(assetPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("opening asset %s: %w", assetPath, err)
	}
	defer f.Close() //nolint:errcheck

	name := fmt.Sprintf("%s", assetPath)
	opts := &github.UploadOptions{Name: name}
	_, _, err = c.gh.Repositories.UploadReleaseAsset(ctx, c.owner, c.repo, releaseID, opts, f)
	if err != nil {
		return fmt.Errorf("uploading asset %s: %w", assetPath, err)
	}
	return nil
}

// BuildReleaseNotes formats commit subjects into a markdown release notes body.
func BuildReleaseNotes(commits []string) string {
	if len(commits) == 0 {
		return ""
	}
	var out string
	for _, c := range commits {
		out += "- " + c + "\n"
	}
	return out
}
