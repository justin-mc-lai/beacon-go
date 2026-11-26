# Beacon Go

**English** | [简体中文](README.zh-CN.md)

Beacon Go is a single-binary **requirement-truth kernel**. It turns one
utterance into an intent tree, freezes nodes only after they have executable
verification, records implementation evidence, rolls verification up the tree,
and releases only with a human nod.

The tree is the only requirement truth:

```
intent -> module -> feature -> touchpoint
```

Every node has an immutable id, a canonical JSON body, a `tree-json-v1`
fingerprint, and a five-state lifecycle. Sea/lake is retired; old packages
import as `feature` nodes.

Current release: **v0.0.1** — the redesigned kernel's first version under the
AGENTS.md truth contract (plain SemVer, see [version-scheme.md](docs/version-scheme.md)).
Release history: [CHANGELOG.md](CHANGELOG.md).

## Why a rewrite

The Python Beacon (v1.6.19, 77 top-level commands) worked, but:

- Its gates were *advisory* — they relied on the host agent's prompt
  discipline instead of executable checks.
- Its surface had grown duplicates: three status views, three scoring
  kits, two brand names (beacon/loom), a compatibility layer shadowing the
  real commands.
- It was slow: every command recomputed the whole project state from
  files (~1s per invocation before doing any work).

The Go rewrite is governed by a written charter
([`docs/scope-charter.md`](docs/scope-charter.md)). Requirement truth lives
only in the tree JSON plus its rendered Markdown; runtime state lives only in
one SQLite ledger; nothing keeps a private state file.

## What it does

Seven command groups, one tree:

| Group | Surface | What it does |
| --- | --- | --- |
| **plan** | `beacon plan …` | Create and expand the tree: `init` the root, validate a proposal with `expand`, write drafts with `apply`. Expansion provenance (model, prompt hash, tokens, cost) is recorded. |
| **truth** | `beacon truth …` | Freeze nodes bottom-up (`truth node freeze`), keep legacy packages, render the tree (the HTML view is a three-pane workbench: full tree, node requirement, related FSM), and produce immutable snapshots. A leaf needs an AC and a verification spec; a module needs children. |
| **change** | `beacon change …` | Invalidate a frozen subtree: descendants become draft, ancestors become `change_pending`; refreeze bottom-up. Records are immutable. |
| **implement** | `beacon implement …` | The host does the work; `implement report` records the leaf, executor identity, artifact hash, and failure reasons. |
| **verify** | `beacon verify …` | Register evidence under a feature or node scope, verify leaves, roll parents up, audit the tree, and run the former qa gates/scores. |
| **release** | `beacon release …` | Release gate and human confirmation; the tree release gate requires a verified root. |
| **goal** | `beacon goal …` | The single-run driver: creates a stage plan, delegates stage readiness to the tree gate, and completes only with a verifier and human confirmation. |

Cross-cutting machinery:

- **Evidence-bound gates** — verification is a registered artifact plus a
  hash; the kernel never trusts prose.
- **Host-driven execution** — no built-in agent executor and no kernel LLM
  client; the host records work with `goal tick --by`.
- **Immutable history** — frozen fingerprints and version snapshots.
- **Dependency gate** — removed LLM/HTTP/executor surfaces cannot re-enter
  the binary (`scripts/check_kernel_deps.sh`).

## Quick start

Requires Go 1.27+.

```sh
go build -o bin/beacon ./cmd/beacon
./bin/beacon --version          # Beacon CLI v0.0.1
```

A typical tree loop:

```sh
beacon projects register --path /path/to/project --version v0.1.0
beacon plan init  --project . --title "my product" --intent "one sentence"
# company/local-only mode: keep .beacon/ and docs/beacon/ out of git
beacon plan init  --project . --title "my product" --intent "one sentence" --local-only
beacon plan expand --project . --node <id> --children proposal.json   # validate
beacon plan apply  --project . --node <id> --proposal proposal.json   # write drafts
beacon truth node freeze --project . --node <id>                     # leaves first
beacon implement report  --project . --node <leaf> --by dev --artifact proof.txt
beacon verify leaf       --project . --node <leaf> --artifact proof.txt --by qa
beacon verify rollup     --project . --node <parent>
beacon verify audit      --project . --json
```

Build the tree from the root; the kernel blocks a parent freeze until every
child is settled. `proposal.json` is produced by the host agent (see the
`beacon-go-plan` skill).

### Ask before sign (legacy feature packages)

Feature packages still work for `goal` runs. A package cannot freeze just
because it is structurally complete; the intent gate requires the author to
record what the user asked, close open questions, trace every AC to an intent
source, and sign:

```sh
beacon truth ask    --feature my-feature --question "Who is this for?" --recommend "..."
beacon truth answer --feature my-feature --id Q1 --answer "..."
beacon truth intent --feature my-feature --utterance "user words" --goal "..." \
                    --non-goals "..." --constraints "..."
beacon truth confirm --feature my-feature
beacon truth freeze  --feature my-feature
```

### Skills

The binary ships seven skills. Install them into a host skill directory; the
host discovers `beacon-go` plus the per-ability skills.

| Skill | Use it when |
| --- | --- |
| `beacon-go-goal` | one utterance should run as a staged delivery |
| `beacon-go-plan` | planning, tree expansion, review, FSM, research, brownfield intake |
| `beacon-go-truth` | create/sign/freeze/change a requirement package |
| `beacon-go-change` | request a change, invalidate a subtree, refreeze bottom-up |
| `beacon-go-implement` | admission, worktree isolation, implement reports |
| `beacon-go-verify` | evidence, leaf verification, rollup, audit, score, parallel code review |
| `beacon-go-release` | release gate and human confirmation |

```sh
beacon skills list                     # ids, names, one-line purposes
beacon skills show plan                # full SKILL.md
beacon skills hosts                    # detected host directories
beacon skills install --host codex     # write beacon-go-<id> + router
beacon skills install --dir <path>     # explicit target directory
beacon skills verify --host codex      # installed vs embedded drift
```

Installed names are namespaced (`beacon-go-goal`, `beacon-go-verify`, …) so
they never collide with another Beacon install. The generated `beacon-go`
router maps a request to one skill. The former `qa` skill is now
`beacon-go-verify`.

## Verification

- `go test ./... -count=1` — behavior-level suite.
- `bash scripts/check_version_consistency.sh` — one version source.
- `bash scripts/check_kernel_deps.sh` — removed surfaces stay out.
- [`e2e/run_e2e.sh`](e2e/run_e2e.sh) — full chain on the manual lane
  (register → truth package → freeze → baseline → goal run/tick → evidence
  → gates → verifier → complete).
- [`e2e/run_dogfood.sh`](e2e/run_dogfood.sh) — badminton tree: 22 nodes,
  12 leaves, bucket freeze → implement → verify → rollup → change/refreeze
  → two snapshots → HTML → audit.


## Known limits (stated, not hidden)

- The kernel does not run agents. Without a host, a goal run stays parked
  and says so.
- Orchestrator `program_*` internals still compile inside the package but
  have no CLI/API entry; they are dead code awaiting removal.
- Desktop and HTTP surfaces are gone; no UI config remains in the repo.
- Live-model skill evaluation is not in CI; skill evals are structural.

## Documentation

Per-phase implementation reports live in [`docs/`](docs/):
`phase0-inventory` (command census), `scope-charter` (the convergence
charter), `p0`–`p6` phase reports, the two fix reports, and
`stdflow-s1/s2/s3` for the standard-flow and topology groups.
They are written in plain language and state their derivations and
trade-offs explicitly.

## Relationship to Python Beacon

The Python repository (`justin-mc-lai/beacon`) remains the untouched
source of record for the frozen v2 design; this repository implements
that design in Go and supersedes the Python runtime for day-to-day use.
`beacon migrate v1` moves existing projects over.

## License

[MIT](LICENSE) © 2026 Kunqi Lai

## Maintainer

**Kunqi Lai** (<goodlive.lkq1128@gmail.com>)
- GitHub: [@justin-mc-lai](https://github.com/justin-mc-lai)

