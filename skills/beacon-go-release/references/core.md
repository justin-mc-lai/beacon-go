<!-- Source: distilled from Python v1.6.19 skills/beacon/
     beacon-release/ (SKILL.md hard gate + references/modes/
     eval-release.md). Re-expressed for the Go release face. -->

# beacon-release — core reference

## Hard gate (Python original)

Release verdict + evidence. Human gate required. Forbidden:
auto-pass without evidence.

## Two levels of "can this ship"

| level | question | Go surface |
|-------|----------|------------|
| run | may THIS run complete? | `beacon release evaluate` / `beacon release complete` — the composed gate: four earlier gates green + human confirmation + verifier + qa pass |
| version | may THIS VERSION ship? | `beacon release check --project . --version <v>` — the version-level checklist (all packages frozen, gates green, coverage, materials fresh, metadata contract, topology) with profile downgrade |

## Profiles (from prod_ops_v168)

- `full` — everything hard.
- `hotfix` — a small named soft set drops from fail to warn
  (the release is urgent; the warnings stay on record).
- `docs_only` — only docs-consistency + the human release gate
  stay hard; everything else reports as warnings.

A downgrade is a recorded verdict change, never a silent skip:
the check output names which items were downgraded and why.

## After completion

Post-release hooks (hook_code `post_release`) are triggered by
completion: auto_allowed hooks run, manual_confirmed hooks wait
for their explicit `beacon hooks run --confirm`, and the builtin
`beacon-post-release-local-install` hook re-aligns the host skill
install with the shipped skills.
