# Changelog

All notable changes to Beacon Go. Versions are plain SemVer
(`MAJOR.MINOR.PATCH`); see `docs/version-scheme.md`.

## [3.0.3] - 2026-11-25

### Changed

- Skill packages now live under their installed names (`beacon-go-*`), carry
  OpenAI metadata, and validate with the official structure checker.
- Frontmatter no longer uses the unsupported `triggers` field; trigger phrases
  moved into the description used by hosts.
- Evals use the standard `skill_name` / `evals` shape and add
  should-not-trigger coverage.
- `beacon-go-plan` adds `pln-brownfield` for inherited codebases.
- `beacon-go-verify` adds `qa-code-review`: an explicit host-side parallel
  reviewer protocol. The kernel records the aggregate; it does not spawn or
  claim parallel reviewers.

## [3.0.2] - 2026-11-25

### Fixed

- The generated `beacon-go` router skill no longer calls itself a "v2 track";
  it now describes the 3.x requirement-truth kernel.

## [3.0.1] - 2026-11-25

### Fixed

- README (EN/ZH) now document the seven groups, tree workflow, and the seven
  embedded skills; stale v2/desktop/executor sections removed.
- The `qa` skill is renamed `beacon-go-verify`; stale program/ralph/sea-lake
  guidance is removed from skill bodies and references.
- `beacon ui` and the `loop` pointer are removed from the CLI; internal
  maintenance commands are hidden from help.

## [3.0.0] - 2026-11-25

### Breaking

- Requirement truth is now an intent tree (`intent → module → feature → touchpoint`);
  the sea/lake write surface (`goal program`, `truth decompose`, `goal parallel-plan`)
  is removed.
- Built-in agent executors (subprocess / profile / ralph), `delegate-ralph`, the
  HTTP projection, desktop workspace, and the kernel LLM client are removed.
  Stage work is host-driven and recorded with `goal tick --by`.
- `qa` is no longer a top-level command; its capabilities live under `verify`.

### Added

- `plan init/tree/expand/apply`, `truth node freeze`, `change request/list`,
  `implement report`, `verify leaf/rollup/audit`, `truth snapshot`, and
  `doctor rebuild-tree`.
- Canonical JSON tree with `tree-json-v1` fingerprints; SQLite is a rebuildable cache.
- Immutable version snapshots with shared bodies; subtree change cascades to ancestors.
- Node-scoped artifact evidence, parent verification rollup, and tree stage gates wired
  into `goal`.
- Badminton dogfood script and CI gate; kernel dependency gate.

### Changed

- Version is plain SemVer from `internal/version/version.go`; 2.0.0 was never tagged.
- Package/evidence schema v2 adds explicit `scope_kind/scope_id` and backfills v1 rows.

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
