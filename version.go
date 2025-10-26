package pocsag

import (
	"fmt"
	"runtime"
)

// Version information - can be set at build time
var (
	Version    = "2.1.0"
	BuildTime  = "unknown"
	GitCommit  = "unknown"
	Author     = "marcell"
	ProjectURL = "https://pagercast.com"
)

// GetVersionString returns a formatted version string
func GetVersionString() string {
	return fmt.Sprintf("POCSAG-GO v%s", Version)
}

// GetBinaryInfo returns binary architecture and runtime information
func GetBinaryInfo() string {
	return fmt.Sprintf("Architecture: %s/%s | Go Version: %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// GetFullVersionInfo returns detailed version information
func GetFullVersionInfo() string {
	return fmt.Sprintf(`POCSAG-GO v%s
Complete Go implementation of POCSAG pager protocol
Author: %s
Build Time: %s
Git Commit: %s
Architecture: %s/%s
Go Version: %s
GitHub: https://github.com/sqpp/pocsag-golang
Project: PagerCast | %s
`, Version, Author, BuildTime, GitCommit, runtime.GOOS, runtime.GOARCH, runtime.Version(), ProjectURL)
}
