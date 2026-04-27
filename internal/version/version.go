// Package version provides build-time version information.
// Values are injected via ldflags during build:
//
//	go build -ldflags "-X github.com/skrptiq/skrptiq-cli/internal/version.Version=v0.2.0 \
//	  -X github.com/skrptiq/skrptiq-cli/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/skrptiq/skrptiq-cli/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
package version

var (
	// Version is the semantic version (e.g. "v0.2.0"). Set at build time.
	Version = "dev"

	// Commit is the short git commit hash. Set at build time.
	Commit = "unknown"

	// Date is the build date in ISO 8601 format. Set at build time.
	Date = "unknown"
)

// Full returns a formatted version string.
func Full() string {
	if Version == "dev" {
		return "dev (built from source)"
	}
	return Version + " (" + Commit + ", " + Date + ")"
}
