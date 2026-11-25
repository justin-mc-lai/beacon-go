// Package version is the single source of the Beacon Go build version.
//
// Release tags are "v" + Version. Version must be plain SemVer
// MAJOR.MINOR.PATCH; prerelease suffixes are reserved for release
// candidates and must not be used as the ongoing development version.
package version

// Version is the current build version.
//
// History: the old Go track used 2.0.0-go.N prerelease strings. Per the
// 2026-11-24 normalization decision (docs/version-scheme.md), those were
// prereleases of 2.0.0. 2.0.0 was never tagged; 3.0.0 is the first
// stable release of the intent-tree kernel.
const Version = "3.0.7"
