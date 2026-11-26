// Package version is the single source of the Beacon Go build version.
//
// Release tags are "v" + Version. Version must be plain SemVer
// MAJOR.MINOR.PATCH; prerelease suffixes are reserved for release
// candidates and must not be used as the ongoing development version.
package version

// Version is the current build version.
//
// The version axis was reset to 0.0.1 when the product was redesigned from
// scratch under the AGENTS.md "唯一真源" contract. Earlier version lines are
// gone with the historical frozen artifacts.
const Version = "0.0.1"
