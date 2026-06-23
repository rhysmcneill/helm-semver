// Package release creates GitHub Releases via the GitHub API.
package release

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"

	igit "github.com/rhysmcneill/helm-semver/internal/git"
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
	f, err := os.Open(assetPath) // #nosec
	if err != nil {
		return fmt.Errorf("opening asset %s: %w", assetPath, err)
	}
	defer f.Close() //nolint:errcheck

	name := assetPath
	opts := &github.UploadOptions{Name: name}
	_, _, err = c.gh.Repositories.UploadReleaseAsset(ctx, c.owner, c.repo, releaseID, opts, f)
	if err != nil {
		return fmt.Errorf("uploading asset %s: %w", assetPath, err)
	}
	return nil
}

// BuildReleaseNotes formats commits into a markdown release notes body.
// When repoOwner and repoName are non-empty, each entry includes PR and commit
// SHA hyperlinks matching the release-please format.
func BuildReleaseNotes(commits []igit.CommitInfo, repoOwner, repoName string) string {
	if len(commits) == 0 {
		return ""
	}

	repoBase := ""
	if repoOwner != "" && repoName != "" {
		repoBase = fmt.Sprintf("https://github.com/%s/%s", repoOwner, repoName)
	}

	var sb strings.Builder
	for _, c := range commits {
		var links []string
		if repoBase != "" {
			if c.PR > 0 {
				links = append(links, fmt.Sprintf("([#%d](%s/pull/%d))", c.PR, repoBase, c.PR))
			}
			if c.Hash != "" {
				short := c.Hash
				if len(short) > 7 {
					short = short[:7]
				}
				links = append(links, fmt.Sprintf("([%s](%s/commit/%s))", short, repoBase, c.Hash))
			}
		}
		if len(links) > 0 {
			fmt.Fprintf(&sb, "- %s %s\n", c.Subject, strings.Join(links, " "))
		} else {
			fmt.Fprintf(&sb, "- %s\n", c.Subject)
		}
	}
	return sb.String()
}
