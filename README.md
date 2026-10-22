# Beacon Go

**English** | [简体中文](README.zh-CN.md)

Beacon Go is a ground-up rewrite of the Beacon kernel in Go. It keeps the
discipline of the original Python Beacon — *decide clearly, freeze the
requirement truth, implement against it, verify with evidence, release only
with a human nod* — while stripping the redundant, outdated, and
off-mainline surface the Python version accumulated over 1.x.

The result is a single static binary whose core is the **goal workflow
engine**: long-running coding work that plans itself into stages, advances
step by step under hard gates, survives interruptions, and never marks
itself done without independent verification and a human-confirmed release.

Current release: **v2.0.0-go.7** · 243 behavior tests · full end-to-end
chain green.

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
([`docs/scope-charter.md`](docs/scope-charter.md)): 77 commands converge to
**12 kept / 49 merged / 16 dropped**, and the 22 workflows under `goal` are
migrated in full. Requirement truth lives only in Markdown; runtime state
lives only in one SQLite ledger; nothing keeps a private JSON state file.

## What it does

Six first-class abilities, one orchestration entry:

| Ability | Surface | What it does |
| --- | --- | --- |
| **goal** | `beacon goal …` | The orchestration entry. Create a run from one utterance, auto-plan stages (plan → truth → [design] → implement → qa → release), advance with `tick`/`driver`, block with reasons, checkpoint and resume after interruption, complete only at the release gate with a verifier attached and human confirmation. |
| **truth** | `beacon truth …` | Requirement packages (`truth.md` + `tests.md` + `tasks.md` + `evidence.md`): create, freeze, register changes without touching a frozen body, refreeze. Freezing is branch-disciplined (canonical branch only) and only counts once committed. |
| **design** | `beacon design …` | Design-primitive binding for UI work: no binding, no implement. The kernel tracks *whether the rules are bound*, never draws prototypes. |
| **implement** | `beacon implement …` | Three lanes (single / team / ralph loop) behind one admission check and one hand-back standard; real subprocess executors and real git worktrees (satellites merge into the main star only). |
| **qa** | `beacon qa …` | Evidence-based acceptance: formal gate evaluation that persists verdicts, multi-reviewer planner review, and the one unified skill-score engine (scores are health checks — they never pass or block a gate, and never edit a skill). |
| **release** | `beacon release …` | The release gate: all gates green + human confirmation, or it stays blocked with named reasons. |

Cross-cutting machinery:

- **Hard gates, written down** — four gates (truth_freeze, design,
  implement, qa, release) evaluate real facts and persist verdicts to the
  ledger; a truth-integrity gate cross-checks package references.
- **Long-run care** — a driver pushes runs turn by turn (lease-guarded,
  budget-capped, stops at the release gate), a supervisor detects stalls
  from heartbeats, a kill switch halts all drivers, steering changes are
  surfaced until acknowledged.
- **Standard execution flow** — a fixed eight-step pipeline from canonical
  truth authoring to implement resume; skipping a step fails closed.
- **Topology gate** — understands single-repo, monorepo, and
  multi-git-workspace layouts; loads v1.6.7 GitScopeGraphs and projects
  them into BranchContext; admission requires the topology verdict.
- **Protected paths** — satellite work can never merge `.env`, keys, auth,
  payment, or migration files into the main line.
- **Migration & rebuild** — `beacon migrate v1` imports Python-Beacon
  projects idempotently; `beacon doctor rebuild` reconciles the ledger
  with on-disk truth without ever deleting rows.

## Quick start

Requires Go 1.27+.

```sh
go build -o bin/beacon ./cmd/beacon
./bin/beacon --version          # Beacon CLI v2.0.0-go.7
```

Five commands cover most of a day:

```sh
beacon projects register --path /path/to/project   # register a project
beacon truth create --project . --slug my-feature   # scaffold a requirement package
beacon truth freeze --project . --feature my-feature
beacon goal run "add two-factor auth to the login page" --project . --feature my-feature
beacon goal driver run-loop --run-id <run-id>       # push it to the release gate
beacon spine show --project . --feature my-feature  # read the one ledger view
```

Runtime state lives in a single global SQLite database
(`~/.beacon/beacon.db`, WAL). Requirement truth never enters the database —
it stays in the Markdown packages under
`docs/beacon/<version>/features/<slug>/`.

## Verification

- `go test ./...` — 243 behavior-level tests.
- [`e2e/run_e2e.sh`](e2e/run_e2e.sh) — a scripted full chain
  (register → create → freeze → commit baseline → run with a real
  subprocess executor → driver → gates → verify → complete) against the
  real binary; currently 13 steps, exit 0.
- [`docs/ac-matrix.md`](docs/ac-matrix.md) — the frozen v2 acceptance
  criteria (171 ACs) with a final status per line:
  81 tested, 64 implemented, 17 deferred to external conditions,
  9 dropped by charter decision.
- [`bench/`](bench/) — the performance gate. Read paths run 50–200×
  faster than the Python version; the write paths were reworked to batch
  commits (`docs/p6-fix-write-perf.md`). One benchmark line
  (gate-evaluate latency on this shared VM's slow volume) remains above
  its threshold and is reported as-is, not tuned away.

## Known limits (stated, not hidden)

- **ACP** support is probe-level only (initialize + session/new
  handshake). Real session iteration awaits a real ACP agent binary.
- **Vendor executors** (codex / claude CLIs) are wired as command
  profiles; end-to-end verification awaits authenticated binaries.
- Four acceptance items are deferred to external conditions — see the
  matrix and `docs/p6-implementation.md` §6 for the full gap list.

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
