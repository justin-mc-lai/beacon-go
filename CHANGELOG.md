# Changelog

All notable changes to Beacon Go. Versions are plain SemVer
(`MAJOR.MINOR.PATCH`); see `docs/version-scheme.md`.

The version axis was reset to `0.0.1` when the product was redesigned from
scratch under the AGENTS.md "唯一真源" contract. Earlier version lines and
frozen artifacts were removed.

## [Unreleased]

- `plan tree` and `truth tree` can read repository intent trees without a local
  SQLite ledger. Without a registered current version, a sole tree is selected
  automatically and multiple trees require an explicit `--version`.

## [0.0.1] — redesigned kernel baseline

### Added

- AGENTS.md "唯一真源" as the highest-authority contract: intent tree as the
  single requirement truth; seven abilities
  (`plan|truth|change|implement|verify|release|goal`); worktree as a core
  ability; pure `x.y.z` versions; layered delivery verification; kernel never
  calls LLM/agents.
- Regression test pinning the seven-ability set and order in
  `internal/harness.Abilities`.
- `beacon-go-change` skill aligned to the seven abilities.

### Changed

- `change` is the single change entry: `change request`/`list` act on the
  tree, `change legacy` carries the former `truth change`.
- `BuildStageOrder()` is parameterless and returns the fixed five stages
  `plan/truth/implement/qa/release`.
- e2e exercises the worktree core surface (`goal worktree status`) through
  the real CLI.

### Removed

- Design entirely: ability, goal stage, gate type, CLI flag, admission check.
  Guiding gene: complexity belongs in the system, not in the user's hands;
  user experience is paramount.
- All historical frozen artifacts (`docs/beacon/*` version lines, packages,
  legacy evidence/scripts) and the `truth change` standalone command.
