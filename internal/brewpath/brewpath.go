// Package brewpath maps versioned Homebrew Cellar paths to their stable
// "opt" links so persisted paths (launchd plists) survive brew upgrades.
package brewpath

import (
	"os"
	"path/filepath"
	"strings"
)

// Stable returns the Homebrew opt path for a binary living in
// <prefix>/Cellar/<formula>/<version>/<rest>, i.e. <prefix>/opt/<formula>/<rest>,
// when that path exists. Any other path is returned unchanged.
func Stable(path string) string {
	sep := string(filepath.Separator)
	marker := sep + "Cellar" + sep
	i := strings.LastIndex(path, marker)
	if i < 0 {
		return path
	}
	prefix := path[:i]
	parts := strings.SplitN(path[i+len(marker):], sep, 3)
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" {
		return path
	}
	candidate := filepath.Join(prefix, "opt", parts[0], parts[2])
	if info, err := os.Stat(candidate); err != nil || info.IsDir() {
		return path
	}
	return candidate
}
