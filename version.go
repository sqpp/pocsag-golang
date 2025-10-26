package pocsag

import "fmt"

// Version information - can be set at build time
var (
	Version    = "2.0.0"
	BuildTime  = "unknown"
	GitCommit  = "unknown"
	Author     = "marcell"
	ProjectURL = "https://pagercast.com"
)

// GetVersionString returns a formatted version string
func GetVersionString() string {
	return fmt.Sprintf("POCSAG-GO v%s", Version)
}

// GetFullVersionInfo returns detailed version information
func GetFullVersionInfo() string {
	return fmt.Sprintf(`POCSAG-GO v%s
Complete Go implementation of POCSAG pager protocol
Author: %s
Build Time: %s
Git Commit: %s
GitHub: https://github.com/sqpp/pocsag-golang
Project: PagerCast | %s
`, Version, Author, BuildTime, GitCommit, ProjectURL)
}
