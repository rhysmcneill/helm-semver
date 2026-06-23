// Package changelog generates and appends to CHANGELOG.md files per chart.
package changelog

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	igit "github.com/rhysmcneill/helm-semver/internal/git"
)

// RepoInfo holds GitHub repository coordinates for generating hyperlinks.
type RepoInfo struct {
	Owner string
	Name  string
}

// Entry represents a single release entry to append.
type Entry struct {
	Version  string
	Date     time.Time
	PrevTag  string
	NewTag   string
	Repo     RepoInfo
	Added    []igit.CommitInfo
	Fixed    []igit.CommitInfo
	Changed  []igit.CommitInfo
	Breaking []igit.CommitInfo
}

var rePRSuffix = regexp.MustCompile(`\s*\(#\d+\)\s*$`)

// stripPR removes a trailing "(#N)" PR reference from a commit subject so it
// can be displayed separately as a hyperlink.
func stripPR(subject string) string {
	return strings.TrimSpace(rePRSuffix.ReplaceAllString(subject, ""))
}

// renderCommit formats a single commit as a list item with optional PR and
// commit-SHA hyperlinks when repo coordinates are available.
func renderCommit(c igit.CommitInfo, repo RepoInfo) string {
	subject := c.Subject
	if c.PR > 0 {
		subject = stripPR(subject)
	}

	if repo.Owner == "" || repo.Name == "" {
		return fmt.Sprintf("* %s", subject)
	}

	base := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	var links []string

	if c.PR > 0 {
		links = append(links, fmt.Sprintf("([#%d](%s/pull/%d))", c.PR, base, c.PR))
	}
	if c.Hash != "" {
		short := c.Hash
		if len(short) > 7 {
			short = short[:7]
		}
		links = append(links, fmt.Sprintf("([%s](%s/commit/%s))", short, base, c.Hash))
	}

	if len(links) > 0 {
		return fmt.Sprintf("* %s %s", subject, strings.Join(links, " "))
	}
	return fmt.Sprintf("* %s", subject)
}

// parseCommits categorises commit subjects into entry buckets.
// Non-user-facing types (chore, ci, docs, style, test, build) are silently
// excluded — they produce no changelog entry.
func parseCommits(commits []igit.CommitInfo, e *Entry) {
	for _, info := range commits {
		msg := strings.TrimSpace(info.Subject)
		switch {
		case strings.Contains(strings.ToUpper(msg), "BREAKING CHANGE") ||
			isType(msg, "feat", true):
			e.Breaking = append(e.Breaking, info)
		case isType(msg, "feat", false):
			e.Added = append(e.Added, info)
		case isType(msg, "fix", false):
			e.Fixed = append(e.Fixed, info)
		case isType(msg, "perf", false):
			e.Changed = append(e.Changed, info)
		case isType(msg, "refactor", false):
			e.Changed = append(e.Changed, info)
			// chore, ci, docs, style, test, build → excluded from changelog
		}
	}
}

func isType(msg, t string, breaking bool) bool {
	prefix := t + "("
	if breaking {
		return strings.HasPrefix(msg, t+"!:") ||
			strings.HasPrefix(msg, prefix) && strings.Contains(msg[:strings.Index(msg, ":")+1], "!")
	}
	return strings.HasPrefix(msg, t+":") ||
		(strings.HasPrefix(msg, prefix) && !strings.Contains(msg[:strings.Index(msg, ":")+1], "!"))
}

// render formats an Entry into a markdown section string.
func render(e *Entry) string {
	var sb strings.Builder

	// Version heading with a compare link when repo info and tags are available.
	if e.Repo.Owner != "" && e.Repo.Name != "" && e.PrevTag != "" && e.NewTag != "" {
		compareURL := fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s",
			e.Repo.Owner, e.Repo.Name, e.PrevTag, e.NewTag)
		fmt.Fprintf(&sb, "## [%s](%s) (%s)\n", e.Version, compareURL, e.Date.Format("2006-01-02"))
	} else {
		fmt.Fprintf(&sb, "## [%s] - %s\n", e.Version, e.Date.Format("2006-01-02"))
	}

	if len(e.Breaking) > 0 {
		sb.WriteString("\n### ⚠ BREAKING CHANGES\n")
		for _, c := range e.Breaking {
			fmt.Fprintf(&sb, "%s\n", renderCommit(c, e.Repo))
		}
	}
	if len(e.Added) > 0 {
		sb.WriteString("\n### Features\n")
		for _, c := range e.Added {
			fmt.Fprintf(&sb, "%s\n", renderCommit(c, e.Repo))
		}
	}
	if len(e.Fixed) > 0 {
		sb.WriteString("\n### Bug Fixes\n")
		for _, c := range e.Fixed {
			fmt.Fprintf(&sb, "%s\n", renderCommit(c, e.Repo))
		}
	}
	if len(e.Changed) > 0 {
		sb.WriteString("\n### Changed\n")
		for _, c := range e.Changed {
			fmt.Fprintf(&sb, "%s\n", renderCommit(c, e.Repo))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// Append prepends a new release section to the CHANGELOG.md at path,
// creating it if it does not exist. prevTag and newTag are used to generate a
// compare link in the release heading; repo provides the GitHub owner/name for
// all hyperlinks. Pass empty strings / zero RepoInfo to omit links.
func Append(path, version string, date time.Time, commits []igit.CommitInfo, prevTag, newTag string, repo RepoInfo) error {
	e := &Entry{
		Version: version,
		Date:    date,
		PrevTag: prevTag,
		NewTag:  newTag,
		Repo:    repo,
	}
	parseCommits(commits, e)

	section := render(e)

	var existing string
	data, err := os.ReadFile(path) // #nosec
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	if err == nil {
		existing = string(data)
	}

	// Prepend after the title line if one exists, otherwise just prepend.
	const title = "# Changelog\n"
	var out string
	switch {
	case strings.HasPrefix(existing, title):
		out = title + "\n" + section + strings.TrimPrefix(existing, title+"\n")
	case existing == "":
		out = title + "\n" + section
	default:
		out = section + existing
	}

	if err := os.WriteFile(path, []byte(out), 0o644); err != nil { // #nosec
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
