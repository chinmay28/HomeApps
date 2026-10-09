// Package version names the build: vYEAR.MONTH.PATCH, a calendar version whose
// patch number is the repository's commit count.
//
// Year and Month are the release line, set by `make bump-version` when a
// branch opens and never from the build clock, so rebuilding an old tree
// reports what it originally shipped. Patch is stamped by the linker
// (-X …/version.Patch=N, see scripts/version.sh); an unstamped build says 0.
package version

import "fmt"

const (
	Year  = 2026
	Month = 10
)

// Patch is the commit count, set at link time.
var Patch = "0"

// String is the version as shown in the UI, /api/health and `homeapps version`.
func String() string {
	return fmt.Sprintf("v%d.%d.%s", Year, Month, Patch)
}
