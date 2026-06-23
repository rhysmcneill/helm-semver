// Package main is the entry point for the helm-semver CLI.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rhysmcneill/helm-semver/internal/changelog"
	"github.com/rhysmcneill/helm-semver/internal/chart"
	igit "github.com/rhysmcneill/helm-semver/internal/git"
	"github.com/rhysmcneill/helm-semver/internal/registry"
	"github.com/rhysmcneill/helm-semver/internal/release"
	"github.com/rhysmcneill/helm-semver/internal/semver"
	"github.com/rhysmcneill/helm-semver/internal/version"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "helm-semver",
		Short: "Semver release automation for Helm chart monorepos",
		Long: `helm-semver bumps Helm chart versions from conventional commits,
packages and pushes to OCI, ChartMuseum, or GitHub Pages registries,
and optionally generates changelogs and GitHub Releases.`,
	}

	root.AddCommand(newReleaseCmd())
	root.AddCommand(newVersionCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print helm-semver version information",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "helm-semver %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.BuildDate)
		},
	}
}

type releaseOptions struct {
	chartsDir     string
	registry      string
	registryType  string
	registryUser  string
	registryPass  string
	gitPush       bool
	dryRun        bool
	changelog     bool
	githubRelease bool
	gitToken      string
	githubOwner   string
	githubRepo    string
	tagPrefix     string
	authorName    string
	authorEmail   string
}

func newReleaseCmd() *cobra.Command {
	opts := &releaseOptions{}

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Release changed charts based on conventional commits",
		Long: `Scans each chart directory for conventional commits since the last release tag,
bumps the chart version (fix→patch, feat→minor, feat!→major), packages the chart,
and pushes it to the configured registry.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRelease(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.chartsDir, "charts-dir", "charts", "Root directory containing chart subdirectories")
	cmd.Flags().StringVar(&opts.registry, "registry", "", "Registry URL (required)")
	cmd.Flags().StringVar(&opts.registryType, "registry-type", "oci", "Registry type: oci, chartmuseum, github-pages")
	cmd.Flags().StringVar(&opts.registryUser, "registry-username", "", "Registry username")
	cmd.Flags().StringVar(&opts.registryPass, "registry-password", os.Getenv("REGISTRY_PASSWORD"), "Registry password (env: REGISTRY_PASSWORD)")
	cmd.Flags().BoolVar(&opts.gitPush, "git-push", true, "Push version bump commit and tags")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print what would happen without making any changes")
	cmd.Flags().BoolVar(&opts.changelog, "changelog", true, "Append release entry to CHANGELOG.md per chart")
	cmd.Flags().BoolVar(&opts.githubRelease, "github-release", false, "Create a GitHub Release for each chart")
	cmd.Flags().StringVar(&opts.gitToken, "git-token", os.Getenv("GITHUB_TOKEN"), "Token for git push authentication (env: GITHUB_TOKEN)")
	// --github-token is a deprecated alias kept for backwards compatibility.
	cmd.Flags().StringVar(&opts.gitToken, "github-token", os.Getenv("GITHUB_TOKEN"), "Deprecated: use --git-token")
	if err := cmd.Flags().MarkHidden("github-token"); err != nil {
		panic(err)
	}
	cmd.Flags().StringVar(&opts.githubOwner, "github-owner", os.Getenv("GITHUB_REPOSITORY_OWNER"), "GitHub repository owner")
	cmd.Flags().StringVar(&opts.githubRepo, "github-repo", "", "GitHub repository name")
	cmd.Flags().StringVar(&opts.tagPrefix, "tag-prefix", "", "Prefix for git tags, e.g. 'charts/'")
	cmd.Flags().StringVar(&opts.authorName, "git-author-name", "helm-semver[bot]", "Git commit author name")
	cmd.Flags().StringVar(&opts.authorEmail, "git-author-email", "helm-semver[bot]@users.noreply.github.com", "Git commit author email")

	_ = cmd.MarkFlagRequired("registry")

	return cmd
}

func runRelease(cmd *cobra.Command, opts *releaseOptions) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}

	gitClient, err := igit.Open(repoRoot)
	if err != nil {
		return fmt.Errorf("opening git repository: %w", err)
	}

	pub, err := newPublisher(opts)
	if err != nil {
		return fmt.Errorf("initialising publisher: %w", err)
	}

	chartsDir := filepath.Join(repoRoot, opts.chartsDir)
	entries, err := os.ReadDir(chartsDir)
	if err != nil {
		return fmt.Errorf("reading charts dir %s: %w", chartsDir, err)
	}

	released := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chartDir := filepath.Join(chartsDir, entry.Name())
		chartYAML := filepath.Join(chartDir, "Chart.yaml")
		if _, err := os.Stat(chartYAML); os.IsNotExist(err) {
			continue
		}

		if err := releaseChart(cmd, opts, gitClient, pub, repoRoot, chartDir, entry.Name()); err != nil {
			return err
		}
		released++
	}

	if released > 0 && opts.gitPush && !opts.dryRun {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Pushing commits and tags…")
		if err := gitClient.Push("origin", opts.gitToken); err != nil {
			return fmt.Errorf("git push: %w", err)
		}
	}

	return nil
}

func releaseChart(cmd *cobra.Command, opts *releaseOptions, gitClient *igit.Client, pub registry.Publisher, repoRoot, chartDir, chartName string) error {
	out := cmd.OutOrStdout()

	// Resolve current version.
	m, err := chart.Load(filepath.Join(chartDir, "Chart.yaml"))
	if err != nil {
		return fmt.Errorf("loading chart %s: %w", chartName, err)
	}

	// Find last release tag and commits since.
	tagName := opts.tagPrefix + chartName
	lastTag, err := gitClient.LatestTag(tagName)
	if err != nil {
		return fmt.Errorf("resolving latest tag for %s: %w", chartName, err)
	}

	relPath, err := filepath.Rel(repoRoot, chartDir)
	if err != nil {
		return fmt.Errorf("resolving relative path for %s: %w", chartName, err)
	}

	commits, err := gitClient.CommitsSince(lastTag, relPath)
	if err != nil {
		return fmt.Errorf("listing commits for %s: %w", chartName, err)
	}

	bump := semver.Analyze(igit.Subjects(commits))
	if bump == semver.BumpNone {
		_, _ = fmt.Fprintf(out, "  %s: no releasable commits — skipping\n", chartName)
		return nil
	}

	newVersion, err := semver.Next(m.Version, bump)
	if err != nil {
		return fmt.Errorf("computing next version for %s: %w", chartName, err)
	}

	newTag := opts.tagPrefix + chartName + "-v" + newVersion

	_, _ = fmt.Fprintf(out, "  %s: %s → %s (%s)\n", chartName, m.Version, newVersion, bump)

	if opts.dryRun {
		_, _ = fmt.Fprintf(out, "    [dry-run] would push to %s\n", opts.registry)
		_, _ = fmt.Fprintf(out, "    [dry-run] would tag %s\n", newTag)
		if opts.changelog {
			_, _ = fmt.Fprintf(out, "    [dry-run] would update CHANGELOG.md\n")
		}
		if opts.githubRelease {
			_, _ = fmt.Fprintf(out, "    [dry-run] would create GitHub Release %s\n", newTag)
		}
		return nil
	}

	// Bump Chart.yaml.
	if err := chart.BumpVersion(filepath.Join(chartDir, "Chart.yaml"), newVersion); err != nil {
		return fmt.Errorf("bumping version for %s: %w", chartName, err)
	}

	// Push to registry.
	if err := pub.Push(chartDir, newVersion); err != nil {
		return fmt.Errorf("pushing %s: %w", chartName, err)
	}
	_, _ = fmt.Fprintf(out, "    pushed to %s\n", opts.registry)

	// Update changelog.
	if opts.changelog {
		clPath := filepath.Join(chartDir, "CHANGELOG.md")
		repo := changelog.RepoInfo{Owner: opts.githubOwner, Name: opts.githubRepo}
		if err := changelog.Append(clPath, newVersion, time.Now(), commits, lastTag, newTag, repo); err != nil {
			return fmt.Errorf("updating changelog: %w", err)
		}
		if err := gitClient.StageFile(filepath.Join(relPath, "CHANGELOG.md")); err != nil {
			return fmt.Errorf("staging CHANGELOG.md for %s: %w", chartName, err)
		}
	}

	// Stage Chart.yaml and commit.
	if err := gitClient.StageFile(filepath.Join(relPath, "Chart.yaml")); err != nil {
		return fmt.Errorf("staging Chart.yaml for %s: %w", chartName, err)
	}

	commitMsg := fmt.Sprintf("chore(%s): release v%s [skip ci]", chartName, newVersion)
	if err := gitClient.Commit(commitMsg, opts.authorName, opts.authorEmail); err != nil {
		return fmt.Errorf("committing release for %s: %w", chartName, err)
	}

	// Tag.
	if err := gitClient.Tag(newTag); err != nil {
		return fmt.Errorf("tagging %s: %w", newTag, err)
	}
	_, _ = fmt.Fprintf(out, "    tagged %s\n", newTag)

	// GitHub Release.
	if opts.githubRelease && opts.gitToken != "" {
		notes := release.BuildReleaseNotes(commits, opts.githubOwner, opts.githubRepo)
		rc := release.New(opts.gitToken, opts.githubOwner, opts.githubRepo)
		url, err := rc.CreateRelease(context.Background(), newTag, chartName+" "+newVersion, notes)
		if err != nil {
			return fmt.Errorf("creating GitHub release for %s: %w", newTag, err)
		}
		_, _ = fmt.Fprintf(out, "    GitHub Release: %s\n", url)
	}

	return nil
}

func newPublisher(opts *releaseOptions) (registry.Publisher, error) {
	switch strings.ToLower(opts.registryType) {
	case "oci":
		return &registry.OCIPublisher{
			RegistryURL: opts.registry,
			Username:    opts.registryUser,
			Password:    opts.registryPass,
		}, nil
	case "chartmuseum":
		return &registry.ChartMuseumPublisher{
			BaseURL:  opts.registry,
			Username: opts.registryUser,
			Password: opts.registryPass,
		}, nil
	case "github-pages":
		return &registry.GitHubPagesPublisher{
			RepoURL:  opts.registry,
			RepoPath: ".",
		}, nil
	default:
		return nil, fmt.Errorf("unknown registry type %q: must be one of oci, chartmuseum, github-pages", opts.registryType)
	}
}

// findRepoRoot walks up from the current directory to find the git root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository")
		}
		dir = parent
	}
}
