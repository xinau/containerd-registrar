package version

import "runtime"

var (
	// Package is filled at linking time
	Package = "github.com/xinau/containerd-registrar"

	// Version holds the complete version number. Filled in at linking time.
	Version = "unknown"

	// Revision is filled with the VCS (e.g. git) revision being used to build
	// the program at linking time.
	Revision = "unknown"

	// GoVersion is Go tree's version.
	GoVersion = runtime.Version()
)
