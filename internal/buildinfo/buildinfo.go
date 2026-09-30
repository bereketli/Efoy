// Package buildinfo holds version metadata injected at build time with -ldflags.
package buildinfo

var (
	// Version is the release version or git describe output.
	Version = "dev"
	// Commit is the short git commit hash.
	Commit = "none"
)
