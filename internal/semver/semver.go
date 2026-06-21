// Package semver parses conventional commits and computes the next semantic
// version for a Helm chart.
package semver

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// BumpType represents the kind of version increment.
type BumpType int

const (
	// BumpNone means no releasable commits were found.
	BumpNone BumpType = iota
	// BumpPatch is triggered by fix: commits.
	BumpPatch
	// BumpMinor is triggered by feat: commits.
	BumpMinor
	// BumpMajor is triggered by feat! or BREAKING CHANGE commits.
	BumpMajor
)

var (
	reBreaking = regexp.MustCompile(`^[a-z]+(\([^)]+\))?!:`)
	reFeat     = regexp.MustCompile(`^feat(\([^)]+\))?:`)
	reFix      = regexp.MustCompile(`^fix(\([^)]+\))?:`)
)

// Analyze inspects a slice of commit subject lines and returns the most
// significant bump type required.
func Analyze(commits []string) BumpType {
	bump := BumpNone
	for _, msg := range commits {
		msg = strings.TrimSpace(msg)
		if reBreaking.MatchString(msg) || strings.Contains(strings.ToUpper(msg), "BREAKING CHANGE") {
			return BumpMajor
		}
		if reFeat.MatchString(msg) && bump < BumpMinor {
			bump = BumpMinor
		}
		if reFix.MatchString(msg) && bump < BumpPatch {
			bump = BumpPatch
		}
	}
	return bump
}

// Next computes the next version string from a current semver string and a
// BumpType. Returns the new version string and an error if current is invalid.
func Next(current string, bump BumpType) (string, error) {
	v, err := semver.NewVersion(current)
	if err != nil {
		return "", fmt.Errorf("parsing version %q: %w", current, err)
	}

	var next semver.Version
	switch bump {
	case BumpMajor:
		next = v.IncMajor()
	case BumpMinor:
		next = v.IncMinor()
	case BumpPatch:
		next = v.IncPatch()
	default:
		return current, nil
	}

	return next.Original(), nil
}

// String returns a human-readable label for a BumpType.
func (b BumpType) String() string {
	switch b {
	case BumpMajor:
		return "major"
	case BumpMinor:
		return "minor"
	case BumpPatch:
		return "patch"
	default:
		return "none"
	}
}
