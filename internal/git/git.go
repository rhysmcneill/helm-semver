// Package git provides git operations for helm-semver: reading commit history,
// writing version bump commits, tagging, and pushing.
package git

import (
	"fmt"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

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

// CommitsSince returns the commit subject lines for commits that touched
// pathFilter since the given tag. If tag is empty, all commits are returned.
func (c *Client) CommitsSince(tag, pathFilter string) ([]string, error) {
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
		PathFilter: func(path string) bool {
			return strings.HasPrefix(path, pathFilter)
		},
	}

	if since != nil {
		logOpts.From = *since
	}

	iter, err := c.repo.Log(logOpts)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var subjects []string
	first := true
	err = iter.ForEach(func(commit *object.Commit) error {
		// Skip the tagged commit itself when a tag is given.
		if first && since != nil && commit.Hash == *since {
			first = false
			return nil
		}
		first = false
		// Only the subject line (first line of message).
		subject := strings.SplitN(strings.TrimSpace(commit.Message), "\n", 2)[0]
		subjects = append(subjects, subject)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterating commits: %w", err)
	}

	return subjects, nil
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
func (c *Client) Push(remote string) error {
	err := c.repo.Push(&gogit.PushOptions{
		RemoteName: remote,
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("pushing to %s: %w", remote, err)
	}
	return nil
}
