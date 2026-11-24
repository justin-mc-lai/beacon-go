# Changelog

All notable changes to Beacon Go. Versions are plain SemVer
(`MAJOR.MINOR.PATCH`); see `docs/version-scheme.md`.

## [2.0.0] - unreleased candidate

> **Not a published release.** 2.0.0 remains an unreleased candidate until a `v2.0.0` tag, GitHub Release, release notes, and `docs/beacon/v2.0.0/release/` are in place.

### Changed

- Normalized the Go track version from the historical `2.0.0-go.N`
  prerelease series to plain SemVer. `2.0.0-go.1` … `2.0.0-go.17` remain
  historical tags and are treated as prereleases of `2.0.0`.
- Added `internal/version/version.go` as the single version source;
  `cli.Version` and the ACP client info now reference it.
- `e2e/run_e2e.sh` now checks the version format (`vX.Y.Z`) instead of a
  hard-coded release string.

### Docs

- Added `docs/version-scheme.md` (version/tag/release rules).
- Added `docs/sdd-kernel-scope.md` (requirement-truth / SDD kernel scope).
