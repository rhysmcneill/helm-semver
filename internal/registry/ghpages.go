package registry

import (
	"fmt"
	"os"
	"path/filepath"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"helm.sh/helm/v3/pkg/action"
	helmchart "helm.sh/helm/v3/pkg/chart/loader"
	helmrepo "helm.sh/helm/v3/pkg/repo"
)

// GitHubPagesPublisher packages the chart, merges it into the repository's
// existing index.yaml (fetched from the gh-pages branch), and pushes the
// result back.
type GitHubPagesPublisher struct {
	// RepoURL is the HTTPS URL of the chart repository served via GitHub Pages,
	// e.g. "https://my-org.github.io/helm-charts".
	RepoURL string
	// RepoPath is the local path to the git repository.
	RepoPath string
	// Branch is the GitHub Pages branch (default "gh-pages").
	Branch string
}

func (p *GitHubPagesPublisher) branch() string {
	if p.Branch != "" {
		return p.Branch
	}
	return "gh-pages"
}

// Push packages the chart at chartDir, fetches the gh-pages branch, merges
// the new chart into the index, and pushes the updated branch.
func (p *GitHubPagesPublisher) Push(chartDir, version string) error {
	ch, err := helmchart.Load(chartDir)
	if err != nil {
		return fmt.Errorf("loading chart at %s: %w", chartDir, err)
	}

	tmpDir, err := os.MkdirTemp("", "helm-semver-ghpages-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	pkg := action.NewPackage()
	pkg.Destination = tmpDir
	tgzPath, err := pkg.Run(chartDir, nil)
	if err != nil {
		return fmt.Errorf("packaging chart: %w", err)
	}

	tgzName := filepath.Base(tgzPath)
	_ = tgzName

	// Open or clone the gh-pages branch as a worktree.
	repo, err := gogit.PlainOpen(p.RepoPath)
	if err != nil {
		return fmt.Errorf("opening repo at %s: %w", p.RepoPath, err)
	}

	// Resolve the gh-pages branch.
	branchRef := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", p.branch()))
	ref, err := repo.Reference(branchRef, true)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", p.branch(), err)
	}

	// Checkout the gh-pages branch into a temp worktree.
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}
	if err := wt.Checkout(&gogit.CheckoutOptions{Hash: ref.Hash()}); err != nil {
		return fmt.Errorf("checking out %s: %w", p.branch(), err)
	}

	// Load existing index.yaml or create a new one.
	indexPath := filepath.Join(p.RepoPath, "index.yaml")
	var existingIndex *helmrepo.IndexFile
	if _, err := os.Stat(indexPath); err == nil {
		existingIndex, err = helmrepo.LoadIndexFile(indexPath)
		if err != nil {
			return fmt.Errorf("loading existing index.yaml: %w", err)
		}
	} else {
		existingIndex = helmrepo.NewIndexFile()
	}

	// Copy the .tgz into the repo root.
	destTGZ := filepath.Join(p.RepoPath, tgzName)
	if err := copyFile(tgzPath, destTGZ); err != nil {
		return fmt.Errorf("copying chart tgz: %w", err)
	}

	// Generate a fresh index from the new .tgz.
	newIndex, err := helmrepo.IndexDirectory(p.RepoPath, p.RepoURL)
	if err != nil {
		return fmt.Errorf("indexing directory: %w", err)
	}

	// Merge new entries into the existing index.
	existingIndex.Merge(newIndex)
	existingIndex.SortEntries()
	if err := existingIndex.WriteFile(indexPath, 0o644); err != nil {
		return fmt.Errorf("writing index.yaml: %w", err)
	}

	// Stage and commit.
	_, _ = wt.Add("index.yaml")
	_, _ = wt.Add(tgzName)
	_, err = wt.Commit(
		fmt.Sprintf("chore: release %s-%s", ch.Name(), version),
		&gogit.CommitOptions{
			Author: &object.Signature{
				Name:  "helm-semver[bot]",
				Email: "helm-semver[bot]@users.noreply.github.com",
			},
		},
	)
	if err != nil {
		return fmt.Errorf("committing gh-pages update: %w", err)
	}

	return nil
}

// MergeIndex merges src into dst, adding any chart versions not already present.
// This is the pure logic extracted for unit testing.
func MergeIndex(dst, src *helmrepo.IndexFile) {
	dst.Merge(src)
	dst.SortEntries()
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil { // #nosec
		return fmt.Errorf("writing %s: %w", dst, err)
	}
	return nil
}
