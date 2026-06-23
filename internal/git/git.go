// Package git provides git operations for helm-semver: reading commit history,
// writing version bump commits, tagging, and pushing.
package git

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CommitInfo holds data about a single commit.
type CommitInfo struct {
	Subject string // First line of the commit message.
	Hash    string // Full 40-character SHA.
	PR      int    // GitHub PR number parsed from trailing "(#N)", 0 if absent.
}

var rePR = regexp.MustCompile(`\(#(\d+)\)\s*$`)

// parsePR extracts a GitHub PR number from a commit subject line that ends with
// "(#N)", as produced by GitHub's default merge commit message format. Returns 0
// if no such suffix is present.
func parsePR(subject string) int {
	m := rePR.FindStringSubmatch(subject)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Subjects returns the Subject field of each CommitInfo, preserving order.
func Subjects(commits []CommitInfo) []string {
	out := make([]string, len(commits))
	for i, c := range commits {
		out[i] = c.Subject
	}
	return out
}

// Client wraps a go-git repository.
type Client struct {
	repo *gogit.Repository
}

// Open opens the git repository at the given path.
func Open(path string) (*Client, error) {
	repo, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("opening git repo at %s: %w", path, err)
	}
	return &Client{repo: repo}, nil
}

// LatestTag returns the most recent tag matching the pattern "<chart>-v*",
// or an empty string if none exists.
func (c *Client) LatestTag(chart string) (string, error) {
	tags, err := c.repo.Tags()
	if err != nil {
		return "", fmt.Errorf("listing tags: %w", err)
	}

	prefix := chart + "-v"
	var matches []string
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("iterating tags: %w", err)
	}

	if len(matches) == 0 {
		return "", nil
	}

	// Sort lexicographically descending — good enough for semver tags that
	// follow the <chart>-vMAJOR.MINOR.PATCH format.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches[0], nil
}

// CommitsSince returns commits that touched pathFilter since the given tag.
// If tag is empty, all commits are returned. Each CommitInfo carries the commit
// subject, full SHA hash, and any GitHub PR number parsed from a "(#N)" trailer.
func (c *Client) CommitsSince(tag, pathFilter string) ([]CommitInfo, error) {
	var since *plumbing.Hash

	if tag != "" {
		ref, err := c.repo.Tag(tag)
		if err != nil {
			return nil, fmt.Errorf("resolving tag %q: %w", tag, err)
		}
		// Tags can point to tag objects or directly to commits.
		tagObj, err := c.repo.TagObject(ref.Hash())
		if err == nil {
			since = &tagObj.Target
		} else {
			h := ref.Hash()
			since = &h
		}
	}

	logOpts := &gogit.LogOptions{
		Order: gogit.LogOrderCommitterTime,
		PathFilter: func(path string) bool {
			return strings.HasPrefix(path, pathFilter)
		},
	}

	iter, err := c.repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []CommitInfo
	err = iter.ForEach(func(commit *object.Commit) error {
		// Stop when we reach the tagged commit (exclusive).
		if since != nil && commit.Hash == *since {
			return storer.ErrStop
		}
		subject := strings.SplitN(strings.TrimSpace(commit.Message), "\n", 2)[0]
		commits = append(commits, CommitInfo{
			Subject: subject,
			Hash:    commit.Hash.String(),
			PR:      parsePR(subject),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	return commits, nil
}

// StageFile adds a file to the git index.
func (c *Client) StageFile(path string) error {
	wt, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}
	if _, err := wt.Add(path); err != nil {
		return fmt.Errorf("staging %s: %w", path, err)
	}
	return nil
}

// Commit creates a new commit with the given message and author details.
func (c *Client) Commit(message, authorName, authorEmail string) error {
	wt, err := c.repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	opts := &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	}

	if _, err := wt.Commit(message, opts); err != nil {
		return fmt.Errorf("committing: %w", err)
	}
	return nil
}

// Tag creates a lightweight tag at HEAD.
func (c *Client) Tag(name string) error {
	head, err := c.repo.Head()
	if err != nil {
		return fmt.Errorf("resolving HEAD: %w", err)
	}

	_, err = c.repo.CreateTag(name, head.Hash(), nil)
	if err != nil {
		return fmt.Errorf("creating tag %q: %w", name, err)
	}
	return nil
}

// Push pushes the current branch and all tags to the named remote.
// If token is non-empty and the remote URL uses HTTPS, the token is used as
// Basic Auth ("x-access-token" / token). For SSH remotes the token is ignored
// — the host's SSH agent handles authentication instead.
func (c *Client) Push(remote, token string) error {
	opts := &gogit.PushOptions{
		RemoteName: remote,
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	}
	if token != "" && c.remoteIsHTTPS(remote) {
		opts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: token,
		}
	}
	err := c.repo.Push(opts)
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("pushing to %s: %w", remote, err)
	}
	return nil
}

// remoteIsHTTPS returns true when the first URL of the named remote uses
// http:// or https://.  SSH remotes (git@ or ssh://) return false.
func (c *Client) remoteIsHTTPS(name string) bool {
	r, err := c.repo.Remote(name)
	if err != nil || len(r.Config().URLs) == 0 {
		// Can't determine — assume HTTPS so we at least try to auth.
		return true
	}
	u := r.Config().URLs[0]
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}
