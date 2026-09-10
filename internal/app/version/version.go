// Package version exposes build metadata injected at link time via -ldflags.
package version

// Build metadata. Overridden by the linker in release builds:
//
//	-X github.com/melvicsosa/skillman/internal/app/version.Version=v1.2.3
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
