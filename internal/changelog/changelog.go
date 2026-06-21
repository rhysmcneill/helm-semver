// Package changelog generates and appends to CHANGELOG.md files per chart.
package changelog

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Entry represents a single release entry to append.
type Entry struct {
	Version  string
	Date     time.Time
	Added    []string
	Fixed    []string
	Changed  []string
	Breaking []string
}

// parseCommits categorises commit subjects into entry buckets.
// Non-user-facing types (chore, ci, docs, style, test, build) are silently
// excluded — they produce no changelog entry.
func parseCommits(commits []string, e *Entry) {
	for _, msg := range commits {
		msg = strings.TrimSpace(msg)
		switch {
		case strings.Contains(strings.ToUpper(msg), "BREAKING CHANGE") ||
			isType(msg, "feat", true):
			e.Breaking = append(e.Breaking, msg)
		case isType(msg, "feat", false):
			e.Added = append(e.Added, msg)
		case isType(msg, "fix", false):
			e.Fixed = append(e.Fixed, msg)
		case isType(msg, "perf", false):
			e.Changed = append(e.Changed, msg)
		case isType(msg, "refactor", false):
			e.Changed = append(e.Changed, msg)
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
	fmt.Fprintf(&sb, "## [%s] - %s\n", e.Version, e.Date.Format("2006-01-02"))

	if len(e.Breaking) > 0 {
		sb.WriteString("\n### Breaking Changes\n")
		for _, c := range e.Breaking {
			fmt.Fprintf(&sb, "- %s\n", c)
		}
	}
	if len(e.Added) > 0 {
		sb.WriteString("\n### Features\n")
		for _, c := range e.Added {
			fmt.Fprintf(&sb, "- %s\n", c)
		}
	}
	if len(e.Fixed) > 0 {
		sb.WriteString("\n### Fixed\n")
		for _, c := range e.Fixed {
			fmt.Fprintf(&sb, "- %s\n", c)
		}
	}
	if len(e.Changed) > 0 {
		sb.WriteString("\n### Changed\n")
		for _, c := range e.Changed {
			fmt.Fprintf(&sb, "- %s\n", c)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// Append prepends a new release section to the CHANGELOG.md at path,
// creating it if it does not exist.
func Append(path, version string, date time.Time, commits []string) error {
	e := &Entry{Version: version, Date: date}
	parseCommits(commits, e)

	section := render(e)

	var existing string
	data, err := os.ReadFile(path) //nolint:gosec
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
