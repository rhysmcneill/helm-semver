// Package version holds build-time version variables set via ldflags.
package version

// Set at build time via ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
